package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	acp "github.com/coder/acp-go-sdk"
	copilot "github.com/github/copilot-sdk/go"

	"github.com/pubgo/redant"
	agentlineapp "github.com/pubgo/redant/cmds/agentlineapp"
	agentacp "github.com/pubgo/redant/cmds/agentlineapp/acp"
	"github.com/pubgo/redant/cmds/webcmd"
	agentlinemodule "github.com/pubgo/redant/pkg/agentline"
)

var demoRT = newDemoRuntime()

func main() {
	var (
		cliPath         string
		logLevel        string
		workingDir      string
		githubToken     string
		useLoggedInUser bool

		model           string
		reasoningEffort string
		systemMessage   string
		streaming       bool
		autoUserAnswer  string

		prompt    string
		sessionID string
		pingMsg   string

		hydrateSessions      bool
		hydrateTimeout       string
		hydrateMaxScanEvents int64

		deleteSessionID  string
		inspectSessionID string

		acpTurnPrompt       string
		permissionDecision  string
		interactiveQuestion string

		dumpSessionEvents bool
		eventsLimit       int64
		eventsRaw         bool
		eventsOut         string
		eventsView        string
	)

	rootCmd := &redant.Command{
		Use:   "copilot-demo",
		Short: "Copilot SDK + redant + agentline 集成示例。",
		Long:  "演示如何在 redant CLI 中通过 Copilot Go SDK 复用 Copilot CLI 能力，并以 agentline runtime 作为交互入口。",
		Options: redant.OptionSet{
			{Flag: "copilot-cli-path", Description: "Copilot CLI 可执行路径（可选）", Value: redant.StringOf(&cliPath)},
			{Flag: "copilot-log-level", Description: "Copilot CLI 日志级别", Value: redant.StringOf(&logLevel), Default: "error"},
			{Flag: "copilot-cwd", Description: "Copilot CLI 进程工作目录", Value: redant.StringOf(&workingDir)},
			{Flag: "copilot-token", Description: "GitHub Token（可选，优先于已登录用户）", Value: redant.StringOf(&githubToken), Envs: []string{"GITHUB_TOKEN"}},
			{Flag: "copilot-use-logged-in-user", Description: "是否使用已登录用户身份", Value: redant.BoolOf(&useLoggedInUser), Default: "true"},
			{Flag: "model", Description: "会话模型", Value: redant.StringOf(&model), Default: "gpt-5"},
			{Flag: "reasoning-effort", Description: "推理强度(low/medium/high/xhigh)", Value: redant.StringOf(&reasoningEffort)},
			{Flag: "system-message", Description: "追加系统提示词（append 模式）", Value: redant.StringOf(&systemMessage)},
			{Flag: "stream", Description: "启用流式输出", Value: redant.BoolOf(&streaming), Default: "false"},
			{Flag: "auto-user-answer", Description: "ask_user 工具触发时自动回答内容", Value: redant.StringOf(&autoUserAnswer), Default: "继续执行"},
		},
	}

	chatCmd := &redant.Command{
		Use:      "chat",
		Short:    "创建新会话并发送 Prompt。",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "prompt", Shorthand: "p", Description: "要发送给 Copilot 的提示词", Value: redant.StringOf(&prompt), Required: true},
			{Flag: "session-id", Description: "指定会话 ID（可选；提供时按 resume 模式继续会话）", Value: redant.StringOf(&sessionID)},
			{Flag: "dump-events", Description: "打印 GetMessages 事件详情（含 ResumeSession 后事件）", Value: redant.BoolOf(&dumpSessionEvents), Default: "false"},
			{Flag: "events-limit", Description: "最多打印最近 N 条事件（0 表示全部）", Value: redant.Int64Of(&eventsLimit), Default: "80"},
			{Flag: "events-raw", Description: "打印事件 data 的完整 JSON（默认摘要）", Value: redant.BoolOf(&eventsRaw), Default: "false"},
			{Flag: "events-out", Description: "事件导出文件（JSONL，默认 data.jsonl）", Value: redant.StringOf(&eventsOut), Default: "data.jsonl"},
			{Flag: "events-view", Description: "事件展示模式(timeline/summary/none)", Value: redant.StringOf(&eventsView), Default: "timeline"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath: cliPath, logLevel: logLevel, cwd: workingDir, token: githubToken, useLoggedInUser: useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sid := strings.TrimSpace(sessionID)
				var (
					session *copilot.Session
					err     error
				)

				if sid != "" {
					if cached, ok := demoRT.GetSession(sid); ok {
						session = cached
						_, _ = fmt.Fprintf(inv.Stdout, "chat switched to resume mode, reuse cached session: %s\n", sid)
					} else {
						session, err = client.ResumeSession(ctx, sid, &copilot.ResumeSessionConfig{
							Model:               strings.TrimSpace(model),
							ReasoningEffort:     strings.TrimSpace(reasoningEffort),
							Streaming:           streaming,
							OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
							OnUserInputRequest:  buildUserInputHandler(inv, autoUserAnswer),
							Hooks:               buildHooks(inv),
						})
						if err != nil {
							return fmt.Errorf("chat resume session: %w", err)
						}
						demoRT.StoreSession(session)
						_, _ = fmt.Fprintf(inv.Stdout, "chat switched to resume mode, session resumed and cached: %s\n", sid)
					}
					if dumpSessionEvents {
						_ = dumpSessionMessages(ctx, inv, session, "after-resume", int(eventsLimit), eventsRaw, strings.TrimSpace(eventsOut), strings.TrimSpace(eventsView))
					}
				} else {
					tool := newEchoTool(inv)
					session, err = client.CreateSession(ctx, &copilot.SessionConfig{
						Model:               strings.TrimSpace(model),
						ReasoningEffort:     strings.TrimSpace(reasoningEffort),
						Streaming:           streaming,
						Tools:               []copilot.Tool{tool},
						SystemMessage:       buildSystemMessage(systemMessage),
						OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
						OnUserInputRequest:  buildUserInputHandler(inv, autoUserAnswer),
						Hooks:               buildHooks(inv),
					})
					if err != nil {
						return fmt.Errorf("create session: %w", err)
					}
					demoRT.StoreSession(session)
					_, _ = fmt.Fprintf(inv.Stdout, "session created and cached: %s\n", strings.TrimSpace(session.SessionID))
				}

				err = sendPromptAndRender(ctx, inv, session, strings.TrimSpace(prompt), streaming)
				if dumpSessionEvents {
					_ = dumpSessionMessages(ctx, inv, session, "after-prompt", int(eventsLimit), eventsRaw, strings.TrimSpace(eventsOut), strings.TrimSpace(eventsView))
				}
				return err
			})
		},
	}

	resumeCmd := &redant.Command{
		Use:      "resume",
		Short:    "恢复已有会话并继续发送 Prompt。",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "session-id", Description: "待恢复的会话 ID", Value: redant.StringOf(&sessionID), Required: true},
			{Flag: "prompt", Shorthand: "p", Description: "继续发送的提示词", Value: redant.StringOf(&prompt), Default: "继续"},
			{Flag: "dump-events", Description: "打印 GetMessages 事件详情（含 ResumeSession 后事件）", Value: redant.BoolOf(&dumpSessionEvents), Default: "false"},
			{Flag: "events-limit", Description: "最多打印最近 N 条事件（0 表示全部）", Value: redant.Int64Of(&eventsLimit), Default: "80"},
			{Flag: "events-raw", Description: "打印事件 data 的完整 JSON（默认摘要）", Value: redant.BoolOf(&eventsRaw), Default: "false"},
			{Flag: "events-out", Description: "事件导出文件（JSONL，默认 data.jsonl）", Value: redant.StringOf(&eventsOut), Default: "data.jsonl"},
			{Flag: "events-view", Description: "事件展示模式(timeline/summary/none)", Value: redant.StringOf(&eventsView), Default: "timeline"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath: cliPath, logLevel: logLevel, cwd: workingDir, token: githubToken, useLoggedInUser: useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sid := strings.TrimSpace(sessionID)
				var (
					session *copilot.Session
					ok      bool
					err     error
				)

				if session, ok = demoRT.GetSession(sid); ok {
					_, _ = fmt.Fprintf(inv.Stdout, "reuse cached session: %s\n", sid)
				} else {
					session, err = client.ResumeSession(ctx, sid, &copilot.ResumeSessionConfig{
						Model:               strings.TrimSpace(model),
						ReasoningEffort:     strings.TrimSpace(reasoningEffort),
						Streaming:           streaming,
						OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
						OnUserInputRequest:  buildUserInputHandler(inv, autoUserAnswer),
						Hooks:               buildHooks(inv),
					})
					if err != nil {
						return fmt.Errorf("resume session: %w", err)
					}
					demoRT.StoreSession(session)
					_, _ = fmt.Fprintf(inv.Stdout, "session resumed and cached: %s\n", sid)
				}

				if dumpSessionEvents {
					_ = dumpSessionMessages(ctx, inv, session, "after-resume", int(eventsLimit), eventsRaw, strings.TrimSpace(eventsOut), strings.TrimSpace(eventsView))
				}

				promptText := withDefault(strings.TrimSpace(prompt), "请继续")
				err = sendPromptAndRender(ctx, inv, session, promptText, streaming)
				if dumpSessionEvents {
					_ = dumpSessionMessages(ctx, inv, session, "after-prompt", int(eventsLimit), eventsRaw, strings.TrimSpace(eventsOut), strings.TrimSpace(eventsView))
				}
				if err == nil {
					return nil
				}

				if !ok {
					return err
				}

				// 缓存会话失效时，自动回退为一次真实 Resume。
				demoRT.DeleteSession(sid, inv.Stderr)
				session, err = client.ResumeSession(ctx, sid, &copilot.ResumeSessionConfig{
					Model:               strings.TrimSpace(model),
					ReasoningEffort:     strings.TrimSpace(reasoningEffort),
					Streaming:           streaming,
					OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
					OnUserInputRequest:  buildUserInputHandler(inv, autoUserAnswer),
					Hooks:               buildHooks(inv),
				})
				if err != nil {
					return fmt.Errorf("resume session after cache fallback: %w", err)
				}
				demoRT.StoreSession(session)
				_, _ = fmt.Fprintf(inv.Stdout, "cached session invalid, resumed again: %s\n", sid)
				if dumpSessionEvents {
					_ = dumpSessionMessages(ctx, inv, session, "after-resume-fallback", int(eventsLimit), eventsRaw, strings.TrimSpace(eventsOut), strings.TrimSpace(eventsView))
				}
				err = sendPromptAndRender(ctx, inv, session, promptText, streaming)
				if dumpSessionEvents {
					_ = dumpSessionMessages(ctx, inv, session, "after-prompt-fallback", int(eventsLimit), eventsRaw, strings.TrimSpace(eventsOut), strings.TrimSpace(eventsView))
				}
				return err
			})
		},
	}

	listSessionsCmd := &redant.Command{
		Use:      "sessions",
		Short:    "列出会话。",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "hydrate", Description: "尝试恢复会话并提取最近消息摘要", Value: redant.BoolOf(&hydrateSessions), Default: "false"},
			{Flag: "hydrate-timeout", Description: "单个会话补全超时时间", Value: redant.StringOf(&hydrateTimeout), Default: "4s"},
			{Flag: "hydrate-max-events", Description: "每个会话最多扫描的最近事件数", Value: redant.Int64Of(&hydrateMaxScanEvents), Default: "50"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath: cliPath, logLevel: logLevel, cwd: workingDir, token: githubToken, useLoggedInUser: useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sessions, err := client.ListSessions(ctx, nil)
				if err != nil {
					return fmt.Errorf("list sessions: %w", err)
				}

				if len(sessions) == 0 {
					_, _ = fmt.Fprintln(inv.Stdout, "暂无会话")
					return nil
				}

				hydrateCfg := hydrateConfig{
					enabled:   hydrateSessions,
					timeout:   parseDurationOrDefault(hydrateTimeout, 4*time.Second),
					maxEvents: int(hydrateMaxScanEvents),
				}

				onlyIDCount := 0
				hydratedCount := 0
				for _, s := range sessions {
					info := hydrateSessionInfo{maxEvents: hydrateCfg.maxEvents}
					if hydrateCfg.enabled {
						hydratedCount++
						info = hydrateSession(ctx, client, strings.TrimSpace(s.SessionID), hydrateCfg)
					}

					line, onlyID := renderSessionLine(s, info)
					if onlyID {
						onlyIDCount++
					}
					_, _ = fmt.Fprintln(inv.Stdout, line)
				}

				if onlyIDCount > 0 {
					_, _ = fmt.Fprintf(inv.Stdout, "\n提示: %d/%d 条会话仅返回 session id；这是上游 CLI 当前返回的数据范围，非本命令解析异常。\n", onlyIDCount, len(sessions))
				}
				if hydrateCfg.enabled {
					_, _ = fmt.Fprintf(inv.Stdout, "提示: hydrate 已尝试补全 %d 条会话（timeout=%s, maxEvents=%d）。\n", hydratedCount, hydrateCfg.timeout.String(), hydrateCfg.maxEvents)
				}
				return nil
			})
		},
	}

	lastSessionCmd := &redant.Command{
		Use:   "last-session",
		Short: "获取最近活跃会话 ID。",
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath: cliPath, logLevel: logLevel, cwd: workingDir, token: githubToken, useLoggedInUser: useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				id, err := client.GetLastSessionID(ctx)
				if err != nil {
					return fmt.Errorf("get last session: %w", err)
				}
				if id == nil || strings.TrimSpace(*id) == "" {
					_, _ = fmt.Fprintln(inv.Stdout, "暂无最近会话")
					return nil
				}
				_, _ = fmt.Fprintln(inv.Stdout, *id)
				return nil
			})
		},
	}

	deleteSessionCmd := &redant.Command{
		Use:   "delete-session",
		Short: "删除指定会话。",
		Options: redant.OptionSet{
			{Flag: "session-id", Description: "待删除会话 ID", Value: redant.StringOf(&deleteSessionID), Required: true},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath: cliPath, logLevel: logLevel, cwd: workingDir, token: githubToken, useLoggedInUser: useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sid := strings.TrimSpace(deleteSessionID)
				if err := client.DeleteSession(ctx, sid); err != nil {
					return fmt.Errorf("delete session: %w", err)
				}
				demoRT.DeleteSession(sid, inv.Stderr)
				_, _ = fmt.Fprintf(inv.Stdout, "已删除会话: %s\n", sid)
				return nil
			})
		},
	}

	modelsCmd := &redant.Command{
		Use:   "models",
		Short: "列出可用模型。",
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath: cliPath, logLevel: logLevel, cwd: workingDir, token: githubToken, useLoggedInUser: useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				models, err := client.ListModels(ctx)
				if err != nil {
					return fmt.Errorf("list models: %w", err)
				}
				if len(models) == 0 {
					_, _ = fmt.Fprintln(inv.Stdout, "无可用模型")
					return nil
				}
				for _, m := range models {
					_, _ = fmt.Fprintf(inv.Stdout, "- %s (%s)\n", m.ID, m.Name)
				}
				return nil
			})
		},
	}

	statusCmd := &redant.Command{
		Use:   "status",
		Short: "查看连接、认证与状态信息。",
		Options: redant.OptionSet{
			{Flag: "ping-message", Description: "Ping 消息", Value: redant.StringOf(&pingMsg), Default: "copilot-demo ping"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath: cliPath, logLevel: logLevel, cwd: workingDir, token: githubToken, useLoggedInUser: useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				resp, err := client.Ping(ctx, strings.TrimSpace(pingMsg))
				if err != nil {
					return fmt.Errorf("ping: %w", err)
				}
				_, _ = fmt.Fprintf(inv.Stdout, "ping: message=%q timestamp=%d\n", resp.Message, resp.Timestamp)

				status, err := client.GetStatus(ctx)
				if err == nil {
					_, _ = fmt.Fprintf(inv.Stdout, "status: version=%s protocol=%d\n", status.Version, status.ProtocolVersion)
				}

				auth, err := client.GetAuthStatus(ctx)
				if err == nil {
					_, _ = fmt.Fprintf(inv.Stdout, "auth: isAuthenticated=%v\n", auth.IsAuthenticated)
				}

				return nil
			})
		},
	}

	interactiveDemoCmd := &redant.Command{
		Use:      "interactive-demo",
		Short:    "演示命令与 agentline 的双向交互（Emit + Ask）。",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "question", Description: "要向用户提问的问题", Value: redant.StringOf(&interactiveQuestion), Default: "是否继续执行下一步？"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			bridge, ok := agentlineapp.InteractionFromInvocation(inv)
			if !ok || bridge == nil {
				_, _ = fmt.Fprintln(inv.Stdout, "interactive bridge 不可用：请在 agentline 模式中运行该命令（例如 /interactive-demo）。")
				return nil
			}

			_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "system", Title: "interactive-demo", Lines: []string{"开始演示双向交互协议 v1"}})
			_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "assistant", Title: "assistant", Lines: []string{"我将发起一个问题，请用 /reply 回答。"}})

			resp, err := bridge.Ask(ctx, agentlineapp.AskRequest{Prompt: strings.TrimSpace(interactiveQuestion)})
			if err != nil {
				_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "error", Title: "ask.error", Lines: []string{err.Error()}})
				return err
			}

			if resp.Cancelled {
				_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "result", Title: "interactive-demo", Lines: []string{"问题已取消，演示结束。"}})
				return nil
			}

			answer := strings.TrimSpace(resp.Answer)
			if answer == "" {
				answer = "(empty)"
			}
			_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "user", Title: "user.answer", Lines: []string{answer}})
			_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "assistant", Title: "assistant", Lines: []string{"已收到你的回复，将继续执行。"}})
			_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "result", Title: "interactive-demo", Lines: []string{"双向交互演示完成。"}})
			return nil
		},
	}

	eventsCmd := &redant.Command{
		Use:      "events",
		Short:    "只读查看会话事件（ResumeSession + GetMessages）。",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "session-id", Description: "待查看的会话 ID", Value: redant.StringOf(&inspectSessionID), Required: true},
			{Flag: "events-limit", Description: "最多打印最近 N 条事件（0 表示全部）", Value: redant.Int64Of(&eventsLimit), Default: "80"},
			{Flag: "events-raw", Description: "打印事件 data 的完整 JSON（默认摘要）", Value: redant.BoolOf(&eventsRaw), Default: "false"},
			{Flag: "events-out", Description: "事件导出文件（JSONL，默认 data.jsonl）", Value: redant.StringOf(&eventsOut), Default: "data.jsonl"},
			{Flag: "events-view", Description: "事件展示模式(timeline/summary/none)", Value: redant.StringOf(&eventsView), Default: "timeline"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath: cliPath, logLevel: logLevel, cwd: workingDir, token: githubToken, useLoggedInUser: useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sid := strings.TrimSpace(inspectSessionID)
				if sid == "" {
					return fmt.Errorf("session-id 不能为空")
				}

				if cached, ok := demoRT.GetSession(sid); ok {
					_, _ = fmt.Fprintf(inv.Stdout, "events uses cached session: %s\n", sid)
					return dumpSessionMessages(ctx, inv, cached, "events", int(eventsLimit), eventsRaw, strings.TrimSpace(eventsOut), strings.TrimSpace(eventsView))
				}

				tmpSession, err := client.ResumeSession(ctx, sid, &copilot.ResumeSessionConfig{
					Model:               strings.TrimSpace(model),
					ReasoningEffort:     strings.TrimSpace(reasoningEffort),
					Streaming:           false,
					OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
					DisableResume:       true,
				})
				if err != nil {
					return fmt.Errorf("events resume session: %w", err)
				}
				defer func() {
					if derr := tmpSession.Disconnect(); derr != nil {
						_, _ = fmt.Fprintf(inv.Stderr, "warn: events disconnect failed: %v\n", derr)
					}
				}()
				_, _ = fmt.Fprintf(inv.Stdout, "events resumed temporary session: %s\n", sid)
				return dumpSessionMessages(ctx, inv, tmpSession, "events", int(eventsLimit), eventsRaw, strings.TrimSpace(eventsOut), strings.TrimSpace(eventsView))
			})
		},
	}

	acpTurnCmd := &redant.Command{
		Use:      "acp-turn",
		Short:    "运行一次 ACP 权限回合演示（显式命令入口）。",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "prompt", Shorthand: "p", Description: "演示回合的用户输入", Value: redant.StringOf(&acpTurnPrompt), Default: "请执行一次需要权限确认的操作"},
			{Flag: "permission-decision", Description: "权限决策(allow/deny/cancel)", Value: redant.StringOf(&permissionDecision), Default: "allow"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			stopReason, updates, err := runACPTurnDemo(ctx, strings.TrimSpace(acpTurnPrompt), strings.TrimSpace(permissionDecision))
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(inv.Stdout, "acp stop reason: %s\n", strings.TrimSpace(string(stopReason)))
			if len(updates) == 0 {
				_, _ = fmt.Fprintln(inv.Stdout, "(no acp updates)")
				return nil
			}

			for i, n := range updates {
				blocks := agentacp.RenderSessionNotification(n)
				for _, b := range blocks {
					_, _ = fmt.Fprintf(inv.Stdout, "[%02d][%s/%s]\n", i+1, strings.TrimSpace(b.Kind), strings.TrimSpace(b.Title))
					for _, line := range b.Lines {
						_, _ = fmt.Fprintf(inv.Stdout, "  %s\n", strings.TrimSpace(line))
					}
				}
			}
			return nil
		},
	}

	rootCmd.Children = []*redant.Command{
		chatCmd,
		resumeCmd,
		interactiveDemoCmd,
		eventsCmd,
		listSessionsCmd,
		lastSessionCmd,
		deleteSessionCmd,
		modelsCmd,
		statusCmd,
		acpTurnCmd,
		webcmd.New(),
	}

	rootCmd.Handler = func(ctx context.Context, inv *redant.Invocation) error {
		return agentlineapp.Run(ctx, rootCmd, &agentlineapp.RuntimeOptions{
			Prompt: "agent> ",
			Stdin:  inv.Stdin,
			Stdout: inv.Stdout,
		})
	}

	err := rootCmd.Invoke().WithOS().Run()
	demoRT.Close(os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type clientOptions struct {
	cliPath         string
	logLevel        string
	cwd             string
	token           string
	useLoggedInUser bool
}

func (o clientOptions) key() string {
	return strings.Join([]string{
		strings.TrimSpace(o.cliPath),
		withDefault(strings.TrimSpace(o.logLevel), "error"),
		strings.TrimSpace(o.cwd),
		strings.TrimSpace(o.token),
		fmt.Sprintf("%t", o.useLoggedInUser),
	}, "|")
}

type demoRuntime struct {
	mu        sync.Mutex
	client    *copilot.Client
	clientKey string
	sessions  map[string]*copilot.Session
}

func newDemoRuntime() *demoRuntime {
	return &demoRuntime{sessions: make(map[string]*copilot.Session)}
}

func (r *demoRuntime) ensureClient(ctx context.Context, inv *redant.Invocation, opts clientOptions) (*copilot.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := opts.key()
	if r.client != nil && r.clientKey == key {
		return r.client, nil
	}

	if err := r.closeLocked(inv.Stderr); err != nil {
		_, _ = fmt.Fprintf(inv.Stderr, "warn: close previous copilot runtime failed: %v\n", err)
	}

	client := copilot.NewClient(&copilot.ClientOptions{
		CLIPath:         strings.TrimSpace(opts.cliPath),
		LogLevel:        withDefault(strings.TrimSpace(opts.logLevel), "error"),
		Cwd:             strings.TrimSpace(opts.cwd),
		GitHubToken:     strings.TrimSpace(opts.token),
		UseLoggedInUser: copilot.Bool(opts.useLoggedInUser),
		AutoStart:       copilot.Bool(false),
	})

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("start client: %w", err)
	}

	r.client = client
	r.clientKey = key
	r.sessions = make(map[string]*copilot.Session)
	_, _ = fmt.Fprintln(inv.Stdout, "Copilot client started")
	return r.client, nil
}

func (r *demoRuntime) GetSession(sessionID string) (*copilot.Session, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[sessionID]
	return s, ok
}

func (r *demoRuntime) StoreSession(session *copilot.Session) {
	if session == nil {
		return
	}
	sid := strings.TrimSpace(session.SessionID)
	if sid == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sid] = session
}

func (r *demoRuntime) DeleteSession(sessionID string, stderr io.Writer) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	r.mu.Lock()
	session, ok := r.sessions[sessionID]
	if ok {
		delete(r.sessions, sessionID)
	}
	r.mu.Unlock()

	if ok {
		if err := session.Disconnect(); err != nil && stderr != nil {
			_, _ = fmt.Fprintf(stderr, "warn: disconnect cached session(%s) failed: %v\n", sessionID, err)
		}
	}
}

func (r *demoRuntime) Close(stderr io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.closeLocked(stderr); err != nil && stderr != nil {
		_, _ = fmt.Fprintf(stderr, "warn: close copilot runtime failed: %v\n", err)
	}
}

func (r *demoRuntime) closeLocked(stderr io.Writer) error {
	var closeErr error
	for sid, session := range r.sessions {
		if session == nil {
			delete(r.sessions, sid)
			continue
		}
		if err := session.Disconnect(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("disconnect session(%s): %w", sid, err))
		}
		delete(r.sessions, sid)
	}

	if r.client != nil {
		if err := r.client.Stop(); err != nil {
			if stderr != nil {
				_, _ = fmt.Fprintf(stderr, "warn: stop client failed: %v\n", err)
			}
			closeErr = errors.Join(closeErr, fmt.Errorf("stop client: %w", err))
		}
	}

	r.client = nil
	r.clientKey = ""
	r.sessions = make(map[string]*copilot.Session)
	return closeErr
}

func withClient(ctx context.Context, inv *redant.Invocation, opts clientOptions, fn func(ctx context.Context, client *copilot.Client) error) error {
	client, err := demoRT.ensureClient(ctx, inv, opts)
	if err != nil {
		return err
	}
	return fn(ctx, client)
}

func sendPromptAndRender(ctx context.Context, inv *redant.Invocation, session *copilot.Session, prompt string, stream bool) error {
	if prompt == "" {
		return fmt.Errorf("prompt 不能为空")
	}

	startedAt := time.Now()
	_, _ = fmt.Fprintf(inv.Stdout, "session=%s\n", session.SessionID)
	tracef(inv.Stdout, "send_prompt.start session=%s stream=%v prompt=%q", session.SessionID, stream, compactText(prompt, 200))

	done := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	var eventCount int64

	unsubscribe := session.On(func(event copilot.SessionEvent) {
		seq := atomic.AddInt64(&eventCount, 1)
		tracef(inv.Stdout, "session.event #%d +%s type=%s summary=%s", seq, sinceText(startedAt), event.Type, summarizeSessionEvent(event))

		switch event.Type {
		case "assistant.message_delta", "assistant.reasoning_delta":
			if stream && event.Data.DeltaContent != nil {
				_, _ = fmt.Fprint(inv.Stdout, *event.Data.DeltaContent)
			}
		case "assistant.message":
			if event.Data.Content != nil {
				if stream {
					_, _ = fmt.Fprintln(inv.Stdout)
				}
				_, _ = fmt.Fprintf(inv.Stdout, "assistant: %s\n", *event.Data.Content)
			}
		case "session.error":
			if event.Data.Message != nil {
				select {
				case errCh <- fmt.Errorf("session error: %s", *event.Data.Message):
				default:
				}
			}
		case "session.idle":
			tracef(inv.Stdout, "session.idle observed after %s", sinceText(startedAt))
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	defer func() {
		unsubscribe()
		tracef(inv.Stdout, "send_prompt.unsubscribe session=%s events=%d elapsed=%s", session.SessionID, atomic.LoadInt64(&eventCount), sinceText(startedAt))
	}()

	tracef(inv.Stdout, "send_prompt.dispatch session=%s", session.SessionID)
	if _, err := session.Send(ctx, copilot.MessageOptions{Prompt: prompt}); err != nil {
		tracef(inv.Stdout, "send_prompt.dispatch_failed session=%s err=%v", session.SessionID, err)
		return fmt.Errorf("send prompt: %w", err)
	}
	tracef(inv.Stdout, "send_prompt.dispatched session=%s", session.SessionID)

	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	tracef(inv.Stdout, "send_prompt.wait_idle timeout=%s", (2 * time.Minute).String())

	select {
	case <-done:
		tracef(inv.Stdout, "send_prompt.done session=%s elapsed=%s", session.SessionID, sinceText(startedAt))
		return nil
	case err := <-errCh:
		tracef(inv.Stdout, "send_prompt.error session=%s err=%v elapsed=%s", session.SessionID, err, sinceText(startedAt))
		return err
	case <-waitCtx.Done():
		tracef(inv.Stdout, "send_prompt.timeout session=%s elapsed=%s", session.SessionID, sinceText(startedAt))
		return fmt.Errorf("wait session idle: %w", waitCtx.Err())
	}
}

func runACPTurnDemo(ctx context.Context, prompt, decision string) (acp.StopReason, []acp.SessionNotification, error) {
	prompt = withDefault(strings.TrimSpace(prompt), "请执行一次需要权限确认的操作")
	decision = strings.TrimSpace(strings.ToLower(decision))

	if decision == "" {
		decision = "allow"
	}
	if decision != "allow" && decision != "deny" && decision != "cancel" {
		return "", nil, fmt.Errorf("invalid permission decision %q: must be allow/deny/cancel", decision)
	}

	updates := make([]acp.SessionNotification, 0, 8)
	client := &agentacp.CallbackClient{
		OnSessionUpdate: func(_ context.Context, params acp.SessionNotification) error {
			updates = append(updates, params)
			return nil
		},
		OnRequestPermission: func(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
			switch decision {
			case "allow":
				if optionID, ok := pickPermissionOptionID(params.Options, acp.PermissionOptionKindAllowOnce, acp.PermissionOptionKindAllowAlways); ok {
					return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeSelected(optionID)}, nil
				}
				if len(params.Options) > 0 {
					return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeSelected(params.Options[0].OptionId)}, nil
				}
			case "deny":
				if optionID, ok := pickPermissionOptionID(params.Options, acp.PermissionOptionKindRejectOnce, acp.PermissionOptionKindRejectAlways); ok {
					return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeSelected(optionID)}, nil
				}
			}
			return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
		},
	}

	var bridge *agentacp.AgentBridge
	exec := agentacp.PromptExecutorFunc(func(ctx context.Context, sessionID acp.SessionId, _ []acp.ContentBlock, emit func(update acp.SessionUpdate) error) (acp.StopReason, error) {
		toolID := acp.ToolCallId("call_demo_1")
		title := "demo edit"
		if err := emit(acp.StartToolCall(toolID, title,
			acp.WithStartKind(acp.ToolKindEdit),
			acp.WithStartStatus(acp.ToolCallStatusPending),
		)); err != nil {
			return "", err
		}

		resp, err := bridge.RequestPermission(ctx, acp.RequestPermissionRequest{
			SessionId: sessionID,
			ToolCall: acp.RequestPermissionToolCall{
				ToolCallId: toolID,
				Title:      acp.Ptr(title),
				Kind:       acp.Ptr(acp.ToolKindEdit),
				Status:     acp.Ptr(acp.ToolCallStatusPending),
			},
			Options: []acp.PermissionOption{
				{OptionId: "allow-once", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce},
				{OptionId: "reject-once", Name: "Reject once", Kind: acp.PermissionOptionKindRejectOnce},
			},
		})
		if err != nil {
			return "", err
		}

		if resp.Outcome.Selected == nil || strings.TrimSpace(string(resp.Outcome.Selected.OptionId)) == "" {
			if err := emit(acp.UpdateToolCall(toolID,
				acp.WithUpdateStatus(acp.ToolCallStatusFailed),
				acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolContent(acp.TextBlock("permission denied"))}),
			)); err != nil {
				return "", err
			}
			return acp.StopReasonRefusal, nil
		}

		if err := emit(acp.UpdateToolCall(toolID, acp.WithUpdateStatus(acp.ToolCallStatusInProgress))); err != nil {
			return "", err
		}
		if err := emit(acp.UpdateToolCall(toolID,
			acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
			acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolContent(acp.TextBlock("demo change applied"))}),
		)); err != nil {
			return "", err
		}
		if err := emit(acp.UpdateAgentMessageText("ACP demo done")); err != nil {
			return "", err
		}
		return acp.StopReasonEndTurn, nil
	})

	bridge = agentacp.NewAgentBridge(agentacp.BridgeOptions{Executor: exec, PermissionRequester: client})
	bridge.SetSessionUpdater(client)

	newResp, err := bridge.NewSession(ctx, acp.NewSessionRequest{Cwd: "/tmp", McpServers: nil})
	if err != nil {
		return "", updates, err
	}

	resp, err := bridge.Prompt(ctx, acp.PromptRequest{SessionId: newResp.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock(prompt)}})
	if err != nil {
		return "", updates, err
	}
	return resp.StopReason, updates, nil
}

func pickPermissionOptionID(options []acp.PermissionOption, kinds ...acp.PermissionOptionKind) (acp.PermissionOptionId, bool) {
	for _, kind := range kinds {
		for _, option := range options {
			if option.Kind == kind {
				return option.OptionId, true
			}
		}
	}
	return "", false
}

func buildHooks(inv *redant.Invocation) *copilot.SessionHooks {
	return &copilot.SessionHooks{
		OnSessionStart: func(input copilot.SessionStartHookInput, invocation copilot.HookInvocation) (*copilot.SessionStartHookOutput, error) {
			_, _ = fmt.Fprintf(inv.Stdout, "[hook] session start: source=%s session=%s\n", input.Source, invocation.SessionID)
			tracef(inv.Stdout, "hook.session_start input=%s invocation=%s", compactText(mustJSON(input), 300), compactText(mustJSON(invocation), 300))
			return nil, nil
		},
		OnSessionEnd: func(input copilot.SessionEndHookInput, invocation copilot.HookInvocation) (*copilot.SessionEndHookOutput, error) {
			_, _ = fmt.Fprintf(inv.Stdout, "[hook] session end: reason=%s session=%s\n", input.Reason, invocation.SessionID)
			tracef(inv.Stdout, "hook.session_end input=%s invocation=%s", compactText(mustJSON(input), 300), compactText(mustJSON(invocation), 300))
			return nil, nil
		},
	}
}

func buildSystemMessage(content string) *copilot.SystemMessageConfig {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	return &copilot.SystemMessageConfig{Content: content}
}

func buildUserInputHandler(inv *redant.Invocation, answer string) copilot.UserInputHandler {
	ans := withDefault(strings.TrimSpace(answer), "继续执行")
	return func(request copilot.UserInputRequest, invocation copilot.UserInputInvocation) (copilot.UserInputResponse, error) {
		_, _ = fmt.Fprintf(inv.Stdout, "[ask_user] session=%s question=%s\n", invocation.SessionID, request.Question)
		tracef(inv.Stdout, "ask_user.request session=%s payload=%s", invocation.SessionID, compactText(mustJSON(request), 300))
		tracef(inv.Stdout, "ask_user.response session=%s answer=%q", invocation.SessionID, ans)
		return copilot.UserInputResponse{Answer: ans, WasFreeform: true}, nil
	}
}

func summarizeSessionEvent(event copilot.SessionEvent) string {
	parts := make([]string, 0, 6)
	if event.Data.DeltaContent != nil {
		parts = append(parts, "delta="+compactText(*event.Data.DeltaContent, 120))
	}
	if event.Data.Content != nil {
		parts = append(parts, "content="+compactText(*event.Data.Content, 120))
	}
	if event.Data.Message != nil {
		parts = append(parts, "message="+compactText(*event.Data.Message, 120))
	}
	raw := mustJSON(event.Data)
	if strings.TrimSpace(raw) != "" && raw != "{}" {
		parts = append(parts, "data="+compactText(raw, 280))
	}
	if len(parts) == 0 {
		return "(no-known-fields)"
	}
	return strings.Join(parts, " | ")
}

func dumpSessionMessages(ctx context.Context, inv *redant.Invocation, session *copilot.Session, stage string, limit int, raw bool, outFile, view string) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	if inv == nil || inv.Stdout == nil {
		return fmt.Errorf("invocation stdout is nil")
	}

	events, err := session.GetMessages(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(inv.Stdout, "[events:%s] get messages failed: %v\n", stage, err)
		return err
	}

	total := len(events)
	start := 0
	if limit > 0 && total > limit {
		start = total - limit
	}

	_, _ = fmt.Fprintf(inv.Stdout, "[events:%s] session=%s total=%d showing=%d..%d\n", stage, strings.TrimSpace(session.SessionID), total, start+1, total)

	shown := make([]copilot.SessionEvent, 0, total-start)
	for i := start; i < total; i++ {
		e := events[i]
		shown = append(shown, e)
		summary := summarizeSessionEvent(e)
		_, _ = fmt.Fprintf(inv.Stdout, "[events:%s] #%d type=%s summary=%s\n", stage, i+1, strings.TrimSpace(string(e.Type)), summary)
		if raw {
			_, _ = fmt.Fprintf(inv.Stdout, "[events:%s] #%d data.raw=%s\n", stage, i+1, mustJSON(e.Data))
		}
	}

	outFile = withDefault(strings.TrimSpace(outFile), "data.jsonl")
	if err := appendEventsJSONL(outFile, stage, strings.TrimSpace(session.SessionID), start, shown); err != nil {
		_, _ = fmt.Fprintf(inv.Stdout, "[events:%s] write jsonl failed: %v\n", stage, err)
	} else {
		_, _ = fmt.Fprintf(inv.Stdout, "[events:%s] jsonl appended: %s (%d events)\n", stage, outFile, len(shown))
	}

	renderEventsProcessView(inv.Stdout, stage, start, shown, view)
	return nil
}

type sessionEventRecord struct {
	CapturedAt        string                 `json:"captured_at"`
	Stage             string                 `json:"stage"`
	SessionID         string                 `json:"session_id"`
	SessionEventIndex int                    `json:"session_event_index"`
	Type              string                 `json:"type"`
	Summary           string                 `json:"summary"`
	Data              map[string]interface{} `json:"data"`
}

func appendEventsJSONL(path, stage, sessionID string, startIndex int, events []copilot.SessionEvent) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	now := time.Now().Format(time.RFC3339Nano)
	for i, e := range events {
		record := sessionEventRecord{
			CapturedAt:        now,
			Stage:             stage,
			SessionID:         sessionID,
			SessionEventIndex: startIndex + i + 1,
			Type:              strings.TrimSpace(string(e.Type)),
			Summary:           summarizeSessionEvent(e),
			Data:              eventDataMap(e.Data),
		}
		if err := enc.Encode(record); err != nil {
			return err
		}
	}
	return nil
}

func eventDataMap(v interface{}) map[string]interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]interface{}{"_marshal_error": err.Error(), "_value": fmt.Sprintf("%+v", v)}
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]interface{}{"_unmarshal_error": err.Error(), "_raw": string(b)}
	}
	return out
}

func renderEventsProcessView(w io.Writer, stage string, startIndex int, events []copilot.SessionEvent, view string) {
	if w == nil || len(events) == 0 {
		return
	}
	view = strings.ToLower(strings.TrimSpace(view))
	if view == "" {
		view = "timeline"
	}
	if view == "none" {
		return
	}

	typeCounts := map[string]int{}
	for _, e := range events {
		typeCounts[strings.TrimSpace(string(e.Type))]++
	}

	keys := make([]string, 0, len(typeCounts))
	for k := range typeCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	_, _ = fmt.Fprintf(w, "[events:%s:view] mode=%s total=%d\n", stage, view, len(events))
	_, _ = fmt.Fprintf(w, "[events:%s:view] type-counts:", stage)
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, " %s=%d", k, typeCounts[k])
	}
	_, _ = fmt.Fprintln(w)

	if view == "summary" {
		return
	}

	for i, e := range events {
		idx := startIndex + i + 1
		t := strings.TrimSpace(string(e.Type))
		s := summarizeSessionEvent(e)
		_, _ = fmt.Fprintf(w, "[events:%s:view] #%d %-28s %s\n", stage, idx, t, compactText(s, 220))
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(b)
}

func tracef(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "[trace] "+format+"\n", args...)
}

func sinceText(startedAt time.Time) string {
	if startedAt.IsZero() {
		return "0s"
	}
	return time.Since(startedAt).Truncate(time.Millisecond).String()
}

func newEchoTool(inv *redant.Invocation) copilot.Tool {
	return copilot.Tool{
		Name:           "demo_echo",
		Description:    "Echo input text for demo",
		SkipPermission: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "text to echo"},
			},
			"required": []string{"text"},
		},
		Handler: func(invocation copilot.ToolInvocation) (copilot.ToolResult, error) {
			text := ""
			if m, ok := invocation.Arguments.(map[string]any); ok {
				if v, ok := m["text"]; ok {
					text = strings.TrimSpace(fmt.Sprint(v))
				}
			}
			if text == "" {
				text = "(empty)"
			}

			_, _ = fmt.Fprintf(inv.Stdout, "[tool:demo_echo] session=%s text=%q\n", invocation.SessionID, text)
			return copilot.ToolResult{
				TextResultForLLM: text,
				ResultType:       "success",
				SessionLog:       "demo_echo executed",
				ToolTelemetry:    map[string]any{"tool": "demo_echo"},
			}, nil
		},
	}
}

func withDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func renderSessionLine(s copilot.SessionMetadata, hydrate hydrateSessionInfo) (line string, onlyID bool) {
	parts := []string{fmt.Sprintf("- id=%s", withDefault(strings.TrimSpace(s.SessionID), "(empty)"))}

	if t := strings.TrimSpace(s.StartTime); t != "" {
		parts = append(parts, "start="+t)
	}
	if t := strings.TrimSpace(s.ModifiedTime); t != "" {
		parts = append(parts, "modified="+t)
	}

	if s.Summary != nil {
		summary := strings.TrimSpace(*s.Summary)
		if summary != "" {
			parts = append(parts, "summary="+summary)
		}
	}

	if s.Context != nil {
		if repo := strings.TrimSpace(s.Context.Repository); repo != "" {
			parts = append(parts, "repo="+repo)
		}
		if branch := strings.TrimSpace(s.Context.Branch); branch != "" {
			parts = append(parts, "branch="+branch)
		}
		if cwd := strings.TrimSpace(s.Context.Cwd); cwd != "" {
			parts = append(parts, "cwd="+cwd)
		}
	}

	if hydrate.errorText != "" {
		parts = append(parts, "hydrate.error="+hydrate.errorText)
	}
	if hydrate.messageCount > 0 {
		parts = append(parts, fmt.Sprintf("hydrate.messages=%d", hydrate.messageCount))
	}
	if hydrate.lastAssistant != "" {
		parts = append(parts, "hydrate.assistant="+hydrate.lastAssistant)
	}
	if hydrate.maxEvents > 0 {
		parts = append(parts, fmt.Sprintf("hydrate.scan=%d", hydrate.maxEvents))
	}

	if len(parts) == 1 {
		parts = append(parts, "meta=empty")
		return strings.Join(parts, "  "), true
	}

	return strings.Join(parts, "  "), false
}

type hydrateConfig struct {
	enabled   bool
	timeout   time.Duration
	maxEvents int
}

type hydrateSessionInfo struct {
	messageCount  int
	lastAssistant string
	errorText     string
	maxEvents     int
}

func hydrateSession(ctx context.Context, client *copilot.Client, sessionID string, cfg hydrateConfig) hydrateSessionInfo {
	info := hydrateSessionInfo{maxEvents: cfg.maxEvents}
	if !cfg.enabled || sessionID == "" {
		return info
	}

	if cfg.maxEvents <= 0 {
		cfg.maxEvents = 50
		info.maxEvents = cfg.maxEvents
	}

	rctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	session, err := client.ResumeSession(rctx, sessionID, &copilot.ResumeSessionConfig{
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
		DisableResume:       true,
	})
	if err != nil {
		info.errorText = compactText(err.Error(), 120)
		return info
	}
	defer func() {
		if err := session.Disconnect(); err != nil {
			if strings.TrimSpace(info.errorText) == "" {
				info.errorText = compactText(fmt.Sprintf("disconnect session: %v", err), 120)
			} else {
				info.errorText = compactText(info.errorText+"; disconnect session: "+err.Error(), 120)
			}
		}
	}()

	events, err := session.GetMessages(rctx)
	if err != nil {
		info.errorText = compactText(err.Error(), 120)
		return info
	}

	info.messageCount = len(events)
	start := 0
	if len(events) > cfg.maxEvents {
		start = len(events) - cfg.maxEvents
	}

	for i := len(events) - 1; i >= start; i-- {
		e := events[i]
		if e.Type == "assistant.message" && e.Data.Content != nil {
			text := strings.TrimSpace(*e.Data.Content)
			if text != "" {
				info.lastAssistant = compactText(text, 120)
				break
			}
		}
	}

	return info
}

func compactText(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.Join(strings.Fields(s), " ")
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

func parseDurationOrDefault(raw string, fallback time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
