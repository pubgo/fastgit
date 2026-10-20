// Package acpclient 提供 fastgit 作为 ACP Client 的最小可用实现:
// spawn 外部 ACP agent 子进程,通过 stdio JSON-RPC 完成
// initialize -> session/new -> session/prompt,并收集 agent_message 文本。
package acpclient

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// Agent 描述如何启动一个 ACP agent 子进程。
type Agent struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// Options 控制一次 prompt 轮次。
type Options struct {
	// Cwd 为 session 工作目录,空则使用当前目录。
	Cwd string
	// System 与 Prompt 合并为一条用户消息(ACP 没有独立的 system 通道)。
	System string
	Prompt string
	// Timeout 为整个轮次上限,零值使用 DefaultTimeout。
	Timeout time.Duration
	// OnUpdate 可选,收到每个 session/update 时回调。
	OnUpdate func(update acp.SessionUpdate) error
}

// Result 是一次 prompt 轮次的输出。
type Result struct {
	Text       string
	StopReason acp.StopReason
}

// DefaultTimeout 是单轮 prompt 的默认上限。
const DefaultTimeout = 2 * time.Minute

// AgentName implements acp.Client, collecting agent_message chunks.
type collector struct {
	mu       sync.Mutex
	text     strings.Builder
	denials  []string
	onUpdate func(update acp.SessionUpdate) error
}

var _ acp.Client = (*collector)(nil)

func (c *collector) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsReadTextFile)
}

func (c *collector) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsWriteTextFile)
}

// RequestPermission 自动拒绝:commit 生成场景把所有上下文直接放进 prompt,
// 正常不应触发权限请求;一旦触发说明 agent 想做额外操作,直接 cancel。
func (c *collector) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	title := ""
	if params.ToolCall.Title != nil {
		title = strings.TrimSpace(*params.ToolCall.Title)
	}
	if title == "" {
		title = string(params.ToolCall.ToolCallId)
	}
	c.mu.Lock()
	c.denials = append(c.denials, title)
	c.mu.Unlock()
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}

func (c *collector) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	update := params.Update
	if chunk := update.AgentMessageChunk; chunk != nil && chunk.Content.Text != nil {
		c.mu.Lock()
		c.text.WriteString(chunk.Content.Text.Text)
		c.mu.Unlock()
	}
	if c.onUpdate != nil {
		return c.onUpdate(update)
	}
	return nil
}

func (c *collector) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalCreate)
}

func (c *collector) KillTerminal(ctx context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalKill)
}

func (c *collector) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalOutput)
}

func (c *collector) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalRelease)
}

func (c *collector) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalWaitForExit)
}

func (c *collector) snapshot() (string, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text.String(), append([]string(nil), c.denials...)
}

// Run spawns the agent, performs one prompt turn and returns the collected
// agent_message text. The agent process is always terminated before returning.
func Run(ctx context.Context, agent Agent, opts Options) (Result, error) {
	if strings.TrimSpace(agent.Command) == "" {
		return Result{}, fmt.Errorf("acp agent %q: command is empty", agent.Name)
	}
	prompt := buildPrompt(opts)
	if strings.TrimSpace(prompt) == "" {
		return Result{}, fmt.Errorf("acp agent %q: empty prompt", agent.Name)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cwd := strings.TrimSpace(opts.Cwd)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	cmd := exec.CommandContext(runCtx, agent.Command, agent.Args...)
	cmd.Dir = cwd
	cmd.Env = mergeEnv(os.Environ(), agent.Env)
	cmd.WaitDelay = 5 * time.Second

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Result{}, fmt.Errorf("acp agent %q: stdin pipe: %w", agent.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("acp agent %q: stdout pipe: %w", agent.Name, err)
	}

	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("acp agent %q: start %s: %w", agent.Name, agent.Command, err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	client := &collector{onUpdate: opts.OnUpdate}
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	if _, err := conn.Initialize(runCtx, acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientInfo:         &acp.Implementation{Name: "fastgit", Version: versionString()},
		ClientCapabilities: acp.ClientCapabilities{},
	}); err != nil {
		return Result{}, wrapRunError(agent, "initialize", err, &stderr)
	}

	newSess, err := conn.NewSession(runCtx, acp.NewSessionRequest{Cwd: cwd, McpServers: []acp.McpServer{}})
	if err != nil {
		return Result{}, wrapRunError(agent, "session/new", err, &stderr)
	}

	promptResp, err := conn.Prompt(runCtx, acp.PromptRequest{
		SessionId: newSess.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})
	if err != nil {
		return Result{}, wrapRunError(agent, "session/prompt", err, &stderr)
	}

	text, denials := client.snapshot()
	if len(denials) > 0 {
		return Result{}, fmt.Errorf("acp agent %q requested %d permission(s) which fastgit auto-denied (last: %q); agent message cannot be generated without tool access",
			agent.Name, len(denials), denials[len(denials)-1])
	}
	if strings.TrimSpace(text) == "" {
		return Result{}, fmt.Errorf("acp agent %q returned no agent_message text (stopReason=%s)", agent.Name, promptResp.StopReason)
	}

	return Result{Text: strings.TrimSpace(text), StopReason: promptResp.StopReason}, nil
}

func buildPrompt(opts Options) string {
	system := strings.TrimSpace(opts.System)
	user := strings.TrimSpace(opts.Prompt)
	switch {
	case system == "":
		return user
	case user == "":
		return system
	default:
		return system + "\n\n" + user
	}
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(extra))
	seen := make(map[string]bool, len(extra))
	for k := range extra {
		seen[k] = true
	}
	for _, item := range base {
		name, _, _ := strings.Cut(item, "=")
		if seen[name] {
			continue
		}
		out = append(out, item)
	}
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}

func wrapRunError(agent Agent, stage string, err error, stderr *bytes.Buffer) error {
	msg := strings.TrimSpace(stderr.String())
	if len(msg) > 400 {
		msg = "..." + msg[len(msg)-400:]
	}
	if msg != "" {
		return fmt.Errorf("acp agent %q failed at %s: %w (agent stderr: %s)", agent.Name, stage, err, msg)
	}
	return fmt.Errorf("acp agent %q failed at %s: %w", agent.Name, stage, err)
}
