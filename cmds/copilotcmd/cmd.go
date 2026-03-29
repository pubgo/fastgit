package copilotcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	copilot "github.com/github/copilot-sdk/go"
	agentlineapp "github.com/pubgo/fastgit/cmds/agentlineapp"
	agentlinemodule "github.com/pubgo/fastgit/pkg/agentline"
	"github.com/pubgo/redant"
)

var rt = newRuntime()

func New() *redant.Command {
	var (
		cliPath         string
		logLevel        string
		workingDir      string
		githubToken     string
		useLoggedInUser bool

		model           string
		reasoningEffort string
		streaming       bool
		autoUserAnswer  string

		configDir             string
		sessionWorkingDir     string
		systemMessageMode     string
		systemMessage         string
		systemSectionsJSON    string
		skillDirs             []string
		disabledSkills        []string
		availableTools        []string
		excludedTools         []string
		mcpServersJSON        string
		customAgentsJSON      string
		agentName             string
		customToolsJSON       string
		enableDemoEchoTool    bool
		enableInfiniteSession bool

		prompt    string
		sessionID string
		pingMsg   string
	)

	rootCmd := &redant.Command{
		Use:      "copilot",
		Short:    "通过 fastgit 使用 Copilot CLI 能力",
		Metadata: agentlinemodule.AgentEntryMetadata(),
		Options: redant.OptionSet{
			{Flag: "copilot-cli-path", Description: "Copilot CLI 可执行路径（可选）", Value: redant.StringOf(&cliPath)},
			{Flag: "copilot-log-level", Description: "Copilot CLI 日志级别", Value: redant.StringOf(&logLevel), Default: "error"},
			{Flag: "copilot-cwd", Description: "Copilot CLI 工作目录", Value: redant.StringOf(&workingDir)},
			{Flag: "copilot-token", Description: "GitHub Token（可选）", Value: redant.StringOf(&githubToken), Envs: []string{"GITHUB_TOKEN"}},
			{Flag: "copilot-use-logged-in-user", Description: "是否使用已登录用户身份", Value: redant.BoolOf(&useLoggedInUser), Default: "true"},
			{Flag: "model", Description: "会话模型", Value: redant.StringOf(&model), Default: "gpt-5"},
			{Flag: "reasoning-effort", Description: "推理强度(low/medium/high/xhigh)", Value: redant.StringOf(&reasoningEffort)},
			{Flag: "stream", Description: "启用流式输出", Value: redant.BoolOf(&streaming), Default: "false"},
			{Flag: "auto-user-answer", Description: "ask_user 触发时自动回答内容", Value: redant.StringOf(&autoUserAnswer), Default: "继续执行"},
			{Flag: "config-dir", Description: "覆盖 Copilot Session 的配置目录", Value: redant.StringOf(&configDir)},
			{Flag: "working-directory", Description: "会话工作目录（工具执行根目录）", Value: redant.StringOf(&sessionWorkingDir)},
			{Flag: "system-message-mode", Description: "系统提示词模式(append|replace|customize)", Value: redant.StringOf(&systemMessageMode), Default: "append"},
			{Flag: "system-message", Description: "系统提示词内容", Value: redant.StringOf(&systemMessage)},
			{Flag: "system-sections-json", Description: "customize 模式 section 覆盖 JSON", Value: redant.StringOf(&systemSectionsJSON)},
			{Flag: "skill-dirs", Description: "技能目录列表，可重复传入", Value: redant.StringArrayOf(&skillDirs)},
			{Flag: "disabled-skills", Description: "禁用的技能名列表，可重复传入", Value: redant.StringArrayOf(&disabledSkills)},
			{Flag: "available-tools", Description: "允许工具白名单，可重复传入", Value: redant.StringArrayOf(&availableTools)},
			{Flag: "excluded-tools", Description: "禁用工具黑名单，可重复传入", Value: redant.StringArrayOf(&excludedTools)},
			{Flag: "mcp-servers-json", Description: "MCP servers JSON（map）", Value: redant.StringOf(&mcpServersJSON)},
			{Flag: "custom-agents-json", Description: "自定义 agents JSON（array）", Value: redant.StringOf(&customAgentsJSON)},
			{Flag: "agent", Description: "激活的自定义 agent 名称", Value: redant.StringOf(&agentName)},
			{Flag: "custom-tools-json", Description: "自定义工具 JSON（array）", Value: redant.StringOf(&customToolsJSON)},
			{Flag: "enable-demo-echo-tool", Description: "启用内置 demo_echo 工具", Value: redant.BoolOf(&enableDemoEchoTool), Default: "false"},
			{Flag: "enable-infinite-sessions", Description: "启用 Infinite Sessions", Value: redant.BoolOf(&enableInfiniteSession), Default: "true"},
		},
	}

	chatCmd := &redant.Command{
		Use:      "chat",
		Short:    "创建或复用会话并发送 Prompt",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "prompt", Shorthand: "p", Description: "发送给 Copilot 的提示词", Value: redant.StringOf(&prompt), Required: true},
			{Flag: "session-id", Description: "指定会话 ID（可选）", Value: redant.StringOf(&sessionID)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			adv, err := buildAdvancedConfig(advancedConfigInput{
				SystemMessageMode:     systemMessageMode,
				SystemMessage:         systemMessage,
				SystemSectionsJSON:    systemSectionsJSON,
				SkillDirs:             skillDirs,
				DisabledSkills:        disabledSkills,
				AvailableTools:        availableTools,
				ExcludedTools:         excludedTools,
				MCPServersJSON:        mcpServersJSON,
				CustomAgentsJSON:      customAgentsJSON,
				AgentName:             agentName,
				CustomToolsJSON:       customToolsJSON,
				EnableDemoEchoTool:    enableDemoEchoTool,
				EnableInfiniteSession: enableInfiniteSession,
			})
			if err != nil {
				return err
			}

			return withClient(ctx, inv, clientOptions{cliPath, logLevel, workingDir, githubToken, useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sid := strings.TrimSpace(sessionID)
				if sid == "" {
					s, err := client.CreateSession(ctx, &copilot.SessionConfig{
						Model:               strings.TrimSpace(model),
						ReasoningEffort:     strings.TrimSpace(reasoningEffort),
						ConfigDir:           strings.TrimSpace(configDir),
						WorkingDirectory:    strings.TrimSpace(sessionWorkingDir),
						Tools:               adv.Tools,
						SystemMessage:       adv.SystemMessage,
						AvailableTools:      adv.AvailableTools,
						ExcludedTools:       adv.ExcludedTools,
						MCPServers:          adv.MCPServers,
						CustomAgents:        adv.CustomAgents,
						Agent:               adv.Agent,
						SkillDirectories:    adv.SkillDirectories,
						DisabledSkills:      adv.DisabledSkills,
						InfiniteSessions:    adv.InfiniteSessions,
						Streaming:           streaming,
						OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
						OnUserInputRequest:  buildUserInputHandler(inv, autoUserAnswer),
					})
					if err != nil {
						return fmt.Errorf("create session: %w", err)
					}
					rt.StoreSession(s)
					_, _ = fmt.Fprintf(inv.Stdout, "session created: %s\n", s.SessionID)
					return sendPrompt(ctx, inv, s, strings.TrimSpace(prompt), streaming)
				}

				s, err := ensureSession(ctx, client, sid, resumeOptions{
					Model:              model,
					ReasoningEffort:    reasoningEffort,
					Streaming:          streaming,
					ConfigDir:          configDir,
					WorkingDirectory:   sessionWorkingDir,
					AutoUserAnswer:     autoUserAnswer,
					SystemMessage:      adv.SystemMessage,
					Tools:              adv.Tools,
					AvailableTools:     adv.AvailableTools,
					ExcludedTools:      adv.ExcludedTools,
					MCPServers:         adv.MCPServers,
					CustomAgents:       adv.CustomAgents,
					Agent:              adv.Agent,
					SkillDirectories:   adv.SkillDirectories,
					DisabledSkills:     adv.DisabledSkills,
					InfiniteSessions:   adv.InfiniteSessions,
					PermissionApproval: true,
				}, inv)
				if err != nil {
					return err
				}
				return sendPrompt(ctx, inv, s, strings.TrimSpace(prompt), streaming)
			})
		},
	}

	resumeCmd := &redant.Command{
		Use:      "resume",
		Short:    "恢复会话并继续对话",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "session-id", Description: "待恢复会话 ID", Value: redant.StringOf(&sessionID), Required: true},
			{Flag: "prompt", Shorthand: "p", Description: "继续发送的提示词", Value: redant.StringOf(&prompt), Default: "继续"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			adv, err := buildAdvancedConfig(advancedConfigInput{
				SystemMessageMode:     systemMessageMode,
				SystemMessage:         systemMessage,
				SystemSectionsJSON:    systemSectionsJSON,
				SkillDirs:             skillDirs,
				DisabledSkills:        disabledSkills,
				AvailableTools:        availableTools,
				ExcludedTools:         excludedTools,
				MCPServersJSON:        mcpServersJSON,
				CustomAgentsJSON:      customAgentsJSON,
				AgentName:             agentName,
				CustomToolsJSON:       customToolsJSON,
				EnableDemoEchoTool:    enableDemoEchoTool,
				EnableInfiniteSession: enableInfiniteSession,
			})
			if err != nil {
				return err
			}

			return withClient(ctx, inv, clientOptions{cliPath, logLevel, workingDir, githubToken, useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sid := strings.TrimSpace(sessionID)
				s, err := ensureSession(ctx, client, sid, resumeOptions{
					Model:              model,
					ReasoningEffort:    reasoningEffort,
					Streaming:          streaming,
					ConfigDir:          configDir,
					WorkingDirectory:   sessionWorkingDir,
					AutoUserAnswer:     autoUserAnswer,
					SystemMessage:      adv.SystemMessage,
					Tools:              adv.Tools,
					AvailableTools:     adv.AvailableTools,
					ExcludedTools:      adv.ExcludedTools,
					MCPServers:         adv.MCPServers,
					CustomAgents:       adv.CustomAgents,
					Agent:              adv.Agent,
					SkillDirectories:   adv.SkillDirectories,
					DisabledSkills:     adv.DisabledSkills,
					InfiniteSessions:   adv.InfiniteSessions,
					PermissionApproval: true,
				}, inv)
				if err != nil {
					return err
				}
				return sendPrompt(ctx, inv, s, strings.TrimSpace(prompt), streaming)
			})
		},
	}

	sessionsCmd := &redant.Command{
		Use:      "sessions",
		Short:    "列出 Copilot 会话",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath, logLevel, workingDir, githubToken, useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				items, err := client.ListSessions(ctx, nil)
				if err != nil {
					return fmt.Errorf("list sessions: %w", err)
				}
				if len(items) == 0 {
					_, _ = fmt.Fprintln(inv.Stdout, "暂无会话")
					return nil
				}
				for _, s := range items {
					_, _ = fmt.Fprintf(inv.Stdout, "- %s\n", strings.TrimSpace(s.SessionID))
				}
				return nil
			})
		},
	}

	statusCmd := &redant.Command{
		Use:      "status",
		Short:    "查看连接和认证状态",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options:  redant.OptionSet{{Flag: "ping-message", Description: "Ping 消息", Value: redant.StringOf(&pingMsg), Default: "copilot ping"}},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath, logLevel, workingDir, githubToken, useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				resp, err := client.Ping(ctx, strings.TrimSpace(pingMsg))
				if err != nil {
					return fmt.Errorf("ping: %w", err)
				}
				_, _ = fmt.Fprintf(inv.Stdout, "ping: message=%q timestamp=%d\n", resp.Message, resp.Timestamp)

				if s, err := client.GetStatus(ctx); err == nil {
					_, _ = fmt.Fprintf(inv.Stdout, "status: version=%s protocol=%d\n", s.Version, s.ProtocolVersion)
				}
				if a, err := client.GetAuthStatus(ctx); err == nil {
					_, _ = fmt.Fprintf(inv.Stdout, "auth: isAuthenticated=%v\n", a.IsAuthenticated)
				}
				return nil
			})
		},
	}

	modelsCmd := &redant.Command{
		Use:      "models",
		Short:    "列出可用模型",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			return withClient(ctx, inv, clientOptions{cliPath, logLevel, workingDir, githubToken, useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				models, err := client.ListModels(ctx)
				if err != nil {
					return fmt.Errorf("list models: %w", err)
				}
				for _, m := range models {
					_, _ = fmt.Fprintf(inv.Stdout, "- %s (%s)\n", m.ID, m.Name)
				}
				return nil
			})
		},
	}

	interactiveDemoCmd := &redant.Command{
		Use:      "interactive-demo",
		Short:    "演示命令与 agentline 的双向问答",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			bridge, ok := agentlineapp.InteractionFromInvocation(inv)
			if !ok || bridge == nil {
				_, _ = fmt.Fprintln(inv.Stdout, "interactive bridge 不可用：请在 copilot 交互模式中执行该命令。")
				return nil
			}
			_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "assistant", Title: "interactive", Lines: []string{"请输入 /reply <text> 或直接输入文本回车。"}})
			resp, err := bridge.Ask(ctx, agentlineapp.AskRequest{Prompt: "是否继续执行下一步？"})
			if err != nil {
				return err
			}
			if resp.Cancelled {
				_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "result", Title: "interactive", Lines: []string{"已取消"}})
				return nil
			}
			_ = bridge.Emit(ctx, agentlineapp.InteractionEvent{Kind: "result", Title: "interactive", Lines: []string{"收到回复: " + strings.TrimSpace(resp.Answer)}})
			return nil
		},
	}

	rootCmd.Children = []*redant.Command{chatCmd, resumeCmd, sessionsCmd, statusCmd, modelsCmd, interactiveDemoCmd}
	rootCmd.Handler = func(ctx context.Context, inv *redant.Invocation) error {
		defer rt.Close(inv.Stderr)
		return agentlineapp.Run(ctx, rootCmd, &agentlineapp.RuntimeOptions{Prompt: "copilot> ", Stdin: inv.Stdin, Stdout: inv.Stdout})
	}
	return rootCmd
}

type advancedConfigInput struct {
	SystemMessageMode     string
	SystemMessage         string
	SystemSectionsJSON    string
	SkillDirs             []string
	DisabledSkills        []string
	AvailableTools        []string
	ExcludedTools         []string
	MCPServersJSON        string
	CustomAgentsJSON      string
	AgentName             string
	CustomToolsJSON       string
	EnableDemoEchoTool    bool
	EnableInfiniteSession bool
}

type advancedConfig struct {
	SystemMessage    *copilot.SystemMessageConfig
	SkillDirectories []string
	DisabledSkills   []string
	AvailableTools   []string
	ExcludedTools    []string
	MCPServers       map[string]copilot.MCPServerConfig
	CustomAgents     []copilot.CustomAgentConfig
	Agent            string
	Tools            []copilot.Tool
	InfiniteSessions *copilot.InfiniteSessionConfig
}

func buildAdvancedConfig(in advancedConfigInput) (*advancedConfig, error) {
	sm, err := buildSystemMessageConfig(in.SystemMessageMode, in.SystemMessage, in.SystemSectionsJSON)
	if err != nil {
		return nil, err
	}

	mcp, err := parseMCPServers(in.MCPServersJSON)
	if err != nil {
		return nil, err
	}
	agents, err := parseCustomAgents(in.CustomAgentsJSON)
	if err != nil {
		return nil, err
	}
	tools, err := parseCustomTools(in.CustomToolsJSON)
	if err != nil {
		return nil, err
	}
	if in.EnableDemoEchoTool {
		tools = append(tools, buildDemoEchoTool())
	}

	cfg := &advancedConfig{
		SystemMessage:    sm,
		SkillDirectories: compactStringSlice(in.SkillDirs),
		DisabledSkills:   compactStringSlice(in.DisabledSkills),
		AvailableTools:   compactStringSlice(in.AvailableTools),
		ExcludedTools:    compactStringSlice(in.ExcludedTools),
		MCPServers:       mcp,
		CustomAgents:     agents,
		Agent:            strings.TrimSpace(in.AgentName),
		Tools:            tools,
	}

	if in.EnableInfiniteSession {
		cfg.InfiniteSessions = &copilot.InfiniteSessionConfig{Enabled: copilot.Bool(true)}
	} else {
		cfg.InfiniteSessions = &copilot.InfiniteSessionConfig{Enabled: copilot.Bool(false)}
	}

	return cfg, nil
}

func buildSystemMessageConfig(mode, content, sectionsJSON string) (*copilot.SystemMessageConfig, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "append"
	}

	cfg := &copilot.SystemMessageConfig{Mode: mode, Content: strings.TrimSpace(content)}
	if mode != "customize" {
		if cfg.Content == "" {
			return nil, nil
		}
		return cfg, nil
	}

	if strings.TrimSpace(sectionsJSON) == "" && cfg.Content == "" {
		return nil, nil
	}
	if strings.TrimSpace(sectionsJSON) != "" {
		var sections map[string]copilot.SectionOverride
		if err := json.Unmarshal([]byte(sectionsJSON), &sections); err != nil {
			return nil, fmt.Errorf("invalid --system-sections-json: %w", err)
		}
		cfg.Sections = sections
	}
	return cfg, nil
}

func parseMCPServers(raw string) (map[string]copilot.MCPServerConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out map[string]copilot.MCPServerConfig
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid --mcp-servers-json: %w", err)
	}
	return out, nil
}

func parseCustomAgents(raw string) ([]copilot.CustomAgentConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []copilot.CustomAgentConfig
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid --custom-agents-json: %w", err)
	}
	return out, nil
}

type customToolSpec struct {
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	ResultText     string         `json:"resultText,omitempty"`
	SkipPermission bool           `json:"skipPermission,omitempty"`
	Parameters     map[string]any `json:"parameters,omitempty"`
}

func parseCustomTools(raw string) ([]copilot.Tool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var specs []customToolSpec
	if err := json.Unmarshal([]byte(raw), &specs); err != nil {
		return nil, fmt.Errorf("invalid --custom-tools-json: %w", err)
	}

	tools := make([]copilot.Tool, 0, len(specs))
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			return nil, fmt.Errorf("custom tool name cannot be empty")
		}
		resultText := strings.TrimSpace(spec.ResultText)
		if resultText == "" {
			resultText = "ok"
		}
		parameters := spec.Parameters
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		desc := strings.TrimSpace(spec.Description)
		if desc == "" {
			desc = "custom tool: " + name
		}

		currentName := name
		currentResult := resultText
		tools = append(tools, copilot.Tool{
			Name:           currentName,
			Description:    desc,
			Parameters:     parameters,
			SkipPermission: spec.SkipPermission,
			Handler: func(invocation copilot.ToolInvocation) (copilot.ToolResult, error) {
				return copilot.ToolResult{
					TextResultForLLM: currentResult,
					ResultType:       "success",
					SessionLog:       "tool(" + currentName + ") executed",
					ToolTelemetry: map[string]any{
						"tool":       currentName,
						"session_id": invocation.SessionID,
					},
				}, nil
			},
		})
	}
	return tools, nil
}

func buildDemoEchoTool() copilot.Tool {
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
			text := "(empty)"
			if m, ok := invocation.Arguments.(map[string]any); ok {
				if v, ok := m["text"]; ok {
					t := strings.TrimSpace(fmt.Sprint(v))
					if t != "" {
						text = t
					}
				}
			}
			return copilot.ToolResult{TextResultForLLM: text, ResultType: "success", SessionLog: "demo_echo executed"}, nil
		},
	}
}

func compactStringSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type resumeOptions struct {
	Model              string
	ReasoningEffort    string
	Streaming          bool
	ConfigDir          string
	WorkingDirectory   string
	AutoUserAnswer     string
	SystemMessage      *copilot.SystemMessageConfig
	Tools              []copilot.Tool
	AvailableTools     []string
	ExcludedTools      []string
	MCPServers         map[string]copilot.MCPServerConfig
	CustomAgents       []copilot.CustomAgentConfig
	Agent              string
	SkillDirectories   []string
	DisabledSkills     []string
	InfiniteSessions   *copilot.InfiniteSessionConfig
	PermissionApproval bool
}

func ensureSession(ctx context.Context, client *copilot.Client, sid string, opts resumeOptions, inv *redant.Invocation) (*copilot.Session, error) {
	if cached, ok := rt.GetSession(sid); ok {
		_, _ = fmt.Fprintf(inv.Stdout, "reuse cached session: %s\n", sid)
		return cached, nil
	}

	resumeCfg := &copilot.ResumeSessionConfig{
		Model:            strings.TrimSpace(opts.Model),
		ReasoningEffort:  strings.TrimSpace(opts.ReasoningEffort),
		Streaming:        opts.Streaming,
		ConfigDir:        strings.TrimSpace(opts.ConfigDir),
		WorkingDirectory: strings.TrimSpace(opts.WorkingDirectory),
		SystemMessage:    opts.SystemMessage,
		Tools:            opts.Tools,
		AvailableTools:   opts.AvailableTools,
		ExcludedTools:    opts.ExcludedTools,
		MCPServers:       opts.MCPServers,
		CustomAgents:     opts.CustomAgents,
		Agent:            strings.TrimSpace(opts.Agent),
		SkillDirectories: opts.SkillDirectories,
		DisabledSkills:   opts.DisabledSkills,
		InfiniteSessions: opts.InfiniteSessions,
		OnUserInputRequest: buildUserInputHandler(
			inv,
			opts.AutoUserAnswer,
		),
	}
	if opts.PermissionApproval {
		resumeCfg.OnPermissionRequest = copilot.PermissionHandler.ApproveAll
	}

	s, err := client.ResumeSession(ctx, strings.TrimSpace(sid), resumeCfg)
	if err != nil {
		return nil, fmt.Errorf("resume session: %w", err)
	}
	rt.StoreSession(s)
	_, _ = fmt.Fprintf(inv.Stdout, "session resumed: %s\n", sid)
	return s, nil
}

func sendPrompt(ctx context.Context, inv *redant.Invocation, session *copilot.Session, prompt string, stream bool) error {
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("prompt 不能为空")
	}
	_, _ = fmt.Fprintf(inv.Stdout, "session=%s\n", session.SessionID)

	done := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	unsub := session.On(func(event copilot.SessionEvent) {
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
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	defer unsub()

	if _, err := session.Send(ctx, copilot.MessageOptions{Prompt: strings.TrimSpace(prompt)}); err != nil {
		return fmt.Errorf("send prompt: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	select {
	case <-done:
		return nil
	case err := <-errCh:
		return err
	case <-waitCtx.Done():
		return fmt.Errorf("wait session idle: %w", waitCtx.Err())
	}
}

func buildUserInputHandler(inv *redant.Invocation, answer string) copilot.UserInputHandler {
	ans := strings.TrimSpace(answer)
	if ans == "" {
		ans = "继续执行"
	}
	return func(request copilot.UserInputRequest, invocation copilot.UserInputInvocation) (copilot.UserInputResponse, error) {
		_, _ = fmt.Fprintf(inv.Stdout, "[ask_user] session=%s question=%s\n", invocation.SessionID, request.Question)
		return copilot.UserInputResponse{Answer: ans, WasFreeform: true}, nil
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
		strings.TrimSpace(o.logLevel),
		strings.TrimSpace(o.cwd),
		strings.TrimSpace(o.token),
		fmt.Sprintf("%t", o.useLoggedInUser),
	}, "|")
}

type runtime struct {
	mu        sync.Mutex
	client    *copilot.Client
	clientKey string
	sessions  map[string]*copilot.Session
}

func newRuntime() *runtime {
	return &runtime{sessions: make(map[string]*copilot.Session)}
}

func (r *runtime) ensureClient(ctx context.Context, inv *redant.Invocation, opts clientOptions) (*copilot.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil && r.clientKey == opts.key() {
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
	r.clientKey = opts.key()
	r.sessions = make(map[string]*copilot.Session)
	_, _ = fmt.Fprintln(inv.Stdout, "Copilot client started")
	return client, nil
}

func (r *runtime) GetSession(sessionID string) (*copilot.Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[strings.TrimSpace(sessionID)]
	return s, ok
}

func (r *runtime) StoreSession(s *copilot.Session) {
	if s == nil || strings.TrimSpace(s.SessionID) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[strings.TrimSpace(s.SessionID)] = s
}

func (r *runtime) Close(stderr io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.closeLocked(stderr); err != nil && stderr != nil {
		_, _ = fmt.Fprintf(stderr, "warn: close copilot runtime failed: %v\n", err)
	}
}

func (r *runtime) closeLocked(stderr io.Writer) error {
	var closeErr error
	for sid, s := range r.sessions {
		if s == nil {
			delete(r.sessions, sid)
			continue
		}
		if err := s.Disconnect(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("disconnect session(%s): %w", sid, err))
		}
		delete(r.sessions, sid)
	}
	if r.client != nil {
		if err := r.client.Stop(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("stop client: %w", err))
		}
	}
	r.client = nil
	r.clientKey = ""
	r.sessions = make(map[string]*copilot.Session)
	return closeErr
}

func withClient(ctx context.Context, inv *redant.Invocation, opts clientOptions, fn func(context.Context, *copilot.Client) error) error {
	client, err := rt.ensureClient(ctx, inv, opts)
	if err != nil {
		return err
	}
	return fn(ctx, client)
}

func withDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
