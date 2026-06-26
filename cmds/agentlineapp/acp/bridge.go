package agentacp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
)

// SessionUpdater 定义了向 Client 推送 session/update 的最小能力。
type SessionUpdater interface {
	SessionUpdate(ctx context.Context, params acp.SessionNotification) error
}

// PromptExecutor 由 command 编排层实现，用于驱动多轮/tool 调度。
type PromptExecutor interface {
	ExecutePrompt(ctx context.Context, sessionID acp.SessionId, prompt []acp.ContentBlock, emit func(update acp.SessionUpdate) error) (acp.StopReason, error)
}

// PromptExecutorFunc 允许直接用函数实现 PromptExecutor。
type PromptExecutorFunc func(ctx context.Context, sessionID acp.SessionId, prompt []acp.ContentBlock, emit func(update acp.SessionUpdate) error) (acp.StopReason, error)

// ExecutePrompt implements PromptExecutor.
func (f PromptExecutorFunc) ExecutePrompt(ctx context.Context, sessionID acp.SessionId, prompt []acp.ContentBlock, emit func(update acp.SessionUpdate) error) (acp.StopReason, error) {
	return f(ctx, sessionID, prompt, emit)
}

// PermissionRequester 抽象了 Agent 向 Client 发起 session/request_permission 的能力。
type PermissionRequester interface {
	RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)
}

// BridgeOptions 定义 ACP AgentBridge 的初始化选项。
type BridgeOptions struct {
	AgentInfo           acp.Implementation
	Executor            PromptExecutor
	PermissionRequester PermissionRequester
}

type sessionState struct {
	cancel context.CancelFunc
}

// AgentBridge 是 ACP Agent 侧的最小实现骨架。
//
// 设计目标：
// 1) command 负责编排；
// 2) 通过 session/update 向 UI(Client) 发送结构化事件；
// 3) 支持最小可用生命周期（initialize/new/prompt/cancel）。
type AgentBridge struct {
	mu      sync.Mutex
	updater SessionUpdater

	executor            PromptExecutor
	agentInfo           acp.Implementation
	sessions            map[acp.SessionId]*sessionState
	permissionRequester PermissionRequester
}

// NewAgentBridge 创建一个最小可用的 ACP AgentBridge。
func NewAgentBridge(opts BridgeOptions) *AgentBridge {
	agentInfo := opts.AgentInfo
	if strings.TrimSpace(agentInfo.Name) == "" {
		agentInfo.Name = "redant-agent"
	}
	if strings.TrimSpace(agentInfo.Version) == "" {
		agentInfo.Version = "dev"
	}

	return &AgentBridge{
		executor:            opts.Executor,
		agentInfo:           agentInfo,
		sessions:            make(map[acp.SessionId]*sessionState),
		permissionRequester: opts.PermissionRequester,
	}
}

// RequestPermission 向 Client 发起权限请求；若未配置 requester，默认 cancelled。
func (b *AgentBridge) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	b.mu.Lock()
	requester := b.permissionRequester
	b.mu.Unlock()
	if requester == nil {
		return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
	}
	return requester.RequestPermission(ctx, params)
}

// SetSessionUpdater 设置 session/update 发送目标。
func (b *AgentBridge) SetSessionUpdater(updater SessionUpdater) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.updater = updater
}

// Authenticate implements acp.Agent.
func (b *AgentBridge) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

// Initialize implements acp.Agent.
func (b *AgentBridge) Initialize(_ context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	version := params.ProtocolVersion
	if version != acp.ProtocolVersion(acp.ProtocolVersionNumber) {
		version = acp.ProtocolVersion(acp.ProtocolVersionNumber)
	}

	return acp.InitializeResponse{
		ProtocolVersion: version,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: false,
			PromptCapabilities: acp.PromptCapabilities{
				Audio:           false,
				Image:           false,
				EmbeddedContext: false,
			},
			McpCapabilities: acp.McpCapabilities{Http: false, Sse: false},
		},
		AgentInfo:   &b.agentInfo,
		AuthMethods: []acp.AuthMethod{},
	}, nil
}

// Cancel implements acp.Agent.
func (b *AgentBridge) Cancel(_ context.Context, params acp.CancelNotification) error {
	b.mu.Lock()
	state, ok := b.sessions[params.SessionId]
	if ok && state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}
	b.mu.Unlock()
	return nil
}

// NewSession implements acp.Agent.
func (b *AgentBridge) NewSession(_ context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	if strings.TrimSpace(params.Cwd) == "" {
		return acp.NewSessionResponse{}, acp.NewInvalidParams("cwd is required")
	}
	if !filepath.IsAbs(params.Cwd) {
		return acp.NewSessionResponse{}, acp.NewInvalidParams("cwd must be absolute path")
	}

	sid := acp.SessionId(fmt.Sprintf("sess_%s", strings.ReplaceAll(strings.ToLower(strings.TrimSpace(params.Cwd)), string(filepath.Separator), "_")))
	if strings.TrimSpace(string(sid)) == "sess_" {
		sid = acp.SessionId("sess_default")
	}

	b.mu.Lock()
	b.sessions[sid] = &sessionState{}
	b.mu.Unlock()

	return acp.NewSessionResponse{SessionId: sid}, nil
}

// Prompt implements acp.Agent.
func (b *AgentBridge) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	b.mu.Lock()
	state, ok := b.sessions[params.SessionId]
	b.mu.Unlock()
	if !ok {
		return acp.PromptResponse{}, acp.NewInvalidParams("unknown sessionId")
	}

	promptCtx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	state.cancel = cancel
	b.mu.Unlock()
	defer func() {
		cancel()
		b.mu.Lock()
		if current, exists := b.sessions[params.SessionId]; exists {
			current.cancel = nil
		}
		b.mu.Unlock()
	}()

	emit := func(update acp.SessionUpdate) error {
		return b.emitUpdate(promptCtx, params.SessionId, update)
	}

	for _, block := range params.Prompt {
		if err := emit(acp.UpdateUserMessage(block)); err != nil {
			return acp.PromptResponse{}, err
		}
	}

	if b.executor == nil {
		_ = emit(acp.UpdateAgentMessageText("收到请求，正在由 command 编排层处理。"))
		if errors.Is(promptCtx.Err(), context.Canceled) {
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	}

	stopReason, err := b.executor.ExecutePrompt(promptCtx, params.SessionId, params.Prompt, emit)
	if errors.Is(err, context.Canceled) || errors.Is(promptCtx.Err(), context.Canceled) {
		return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
	}
	if err != nil {
		return acp.PromptResponse{}, err
	}
	if stopReason == "" {
		stopReason = acp.StopReasonEndTurn
	}
	return acp.PromptResponse{StopReason: stopReason}, nil
}

// SetSessionMode implements acp.Agent.
func (b *AgentBridge) SetSessionMode(_ context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	b.mu.Lock()
	_, ok := b.sessions[params.SessionId]
	b.mu.Unlock()
	if !ok {
		return acp.SetSessionModeResponse{}, acp.NewInvalidParams("unknown sessionId")
	}
	return acp.SetSessionModeResponse{}, nil
}

func (b *AgentBridge) emitUpdate(ctx context.Context, sessionID acp.SessionId, update acp.SessionUpdate) error {
	b.mu.Lock()
	updater := b.updater
	b.mu.Unlock()
	if updater == nil {
		return nil
	}
	return updater.SessionUpdate(ctx, acp.SessionNotification{SessionId: sessionID, Update: update})
}
