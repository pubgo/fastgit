package copilotcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	copilot "github.com/github/copilot-sdk/go"
	agentlineapp "github.com/pubgo/fastgit/cmds/agentlineapp"
	agentlinemodule "github.com/pubgo/fastgit/pkg/agentline"
	skillsmodule "github.com/pubgo/fastgit/pkg/skills"
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
		profileName           string
		profileFile           string
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
		enableSkillsTool      bool
		enableInfiniteSession bool

		prompt    string
		sessionID string
		pingMsg   string

		hydrateSessions  bool
		hydrateTimeout   string
		hydrateMaxEvents int64
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
			{Flag: "profile", Description: "使用的配置 profile 名称（从 profile-file 读取）", Value: redant.StringOf(&profileName)},
			{Flag: "profile-file", Description: "profile 配置文件路径（JSON）", Value: redant.StringOf(&profileFile), Default: ".copilot/profiles.json"},
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
			{Flag: "enable-skills-tool", Description: "启用内置 skills_tool function call", Value: redant.BoolOf(&enableSkillsTool), Default: "true"},
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
			resolved, err := resolveCopilotOptions(resolveCopilotInput{
				Model:                 model,
				ReasoningEffort:       reasoningEffort,
				Streaming:             streaming,
				ConfigDir:             configDir,
				SessionWorkingDir:     sessionWorkingDir,
				ProfileName:           profileName,
				ProfileFile:           profileFile,
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
				EnableSkillsTool:      enableSkillsTool,
				EnableInfiniteSession: enableInfiniteSession,
			})
			if err != nil {
				return err
			}

			return withClient(ctx, inv, clientOptions{cliPath, logLevel, workingDir, githubToken, useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sid := strings.TrimSpace(sessionID)
				if sid == "" {
					s, err := client.CreateSession(ctx, &copilot.SessionConfig{
						Model:               strings.TrimSpace(resolved.Model),
						ReasoningEffort:     strings.TrimSpace(resolved.ReasoningEffort),
						ConfigDir:           strings.TrimSpace(resolved.ConfigDir),
						WorkingDirectory:    strings.TrimSpace(resolved.SessionWorkingDir),
						Tools:               resolved.Advanced.Tools,
						SystemMessage:       resolved.Advanced.SystemMessage,
						AvailableTools:      resolved.Advanced.AvailableTools,
						ExcludedTools:       resolved.Advanced.ExcludedTools,
						MCPServers:          resolved.Advanced.MCPServers,
						CustomAgents:        resolved.Advanced.CustomAgents,
						Agent:               resolved.Advanced.Agent,
						SkillDirectories:    resolved.Advanced.SkillDirectories,
						DisabledSkills:      resolved.Advanced.DisabledSkills,
						InfiniteSessions:    resolved.Advanced.InfiniteSessions,
						Streaming:           resolved.Streaming,
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
					Model:              resolved.Model,
					ReasoningEffort:    resolved.ReasoningEffort,
					Streaming:          resolved.Streaming,
					ConfigDir:          resolved.ConfigDir,
					WorkingDirectory:   resolved.SessionWorkingDir,
					AutoUserAnswer:     autoUserAnswer,
					SystemMessage:      resolved.Advanced.SystemMessage,
					Tools:              resolved.Advanced.Tools,
					AvailableTools:     resolved.Advanced.AvailableTools,
					ExcludedTools:      resolved.Advanced.ExcludedTools,
					MCPServers:         resolved.Advanced.MCPServers,
					CustomAgents:       resolved.Advanced.CustomAgents,
					Agent:              resolved.Advanced.Agent,
					SkillDirectories:   resolved.Advanced.SkillDirectories,
					DisabledSkills:     resolved.Advanced.DisabledSkills,
					InfiniteSessions:   resolved.Advanced.InfiniteSessions,
					PermissionApproval: true,
				}, inv)
				if err != nil {
					return err
				}
				return sendPrompt(ctx, inv, s, strings.TrimSpace(prompt), resolved.Streaming)
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
			resolved, err := resolveCopilotOptions(resolveCopilotInput{
				Model:                 model,
				ReasoningEffort:       reasoningEffort,
				Streaming:             streaming,
				ConfigDir:             configDir,
				SessionWorkingDir:     sessionWorkingDir,
				ProfileName:           profileName,
				ProfileFile:           profileFile,
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
				EnableSkillsTool:      enableSkillsTool,
				EnableInfiniteSession: enableInfiniteSession,
			})
			if err != nil {
				return err
			}

			return withClient(ctx, inv, clientOptions{cliPath, logLevel, workingDir, githubToken, useLoggedInUser}, func(ctx context.Context, client *copilot.Client) error {
				sid := strings.TrimSpace(sessionID)
				s, err := ensureSession(ctx, client, sid, resumeOptions{
					Model:              resolved.Model,
					ReasoningEffort:    resolved.ReasoningEffort,
					Streaming:          resolved.Streaming,
					ConfigDir:          resolved.ConfigDir,
					WorkingDirectory:   resolved.SessionWorkingDir,
					AutoUserAnswer:     autoUserAnswer,
					SystemMessage:      resolved.Advanced.SystemMessage,
					Tools:              resolved.Advanced.Tools,
					AvailableTools:     resolved.Advanced.AvailableTools,
					ExcludedTools:      resolved.Advanced.ExcludedTools,
					MCPServers:         resolved.Advanced.MCPServers,
					CustomAgents:       resolved.Advanced.CustomAgents,
					Agent:              resolved.Advanced.Agent,
					SkillDirectories:   resolved.Advanced.SkillDirectories,
					DisabledSkills:     resolved.Advanced.DisabledSkills,
					InfiniteSessions:   resolved.Advanced.InfiniteSessions,
					PermissionApproval: true,
				}, inv)
				if err != nil {
					return err
				}
				return sendPrompt(ctx, inv, s, strings.TrimSpace(prompt), resolved.Streaming)
			})
		},
	}

	sessionsCmd := &redant.Command{
		Use:      "sessions",
		Short:    "列出 Copilot 会话",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "hydrate", Description: "尝试恢复会话并补全最近 assistant 摘要", Value: redant.BoolOf(&hydrateSessions), Default: "false"},
			{Flag: "hydrate-timeout", Description: "单会话补全超时", Value: redant.StringOf(&hydrateTimeout), Default: "4s"},
			{Flag: "hydrate-max-events", Description: "补全时最多扫描的最近事件数", Value: redant.Int64Of(&hydrateMaxEvents), Default: "50"},
		},
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

				hydrateCfg := hydrateConfig{
					enabled:   hydrateSessions,
					timeout:   parseDurationOrDefault(hydrateTimeout, 4*time.Second),
					maxEvents: int(hydrateMaxEvents),
				}

				onlyIDCount := 0
				hydratedCount := 0
				for _, s := range items {
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
					_, _ = fmt.Fprintf(inv.Stdout, "\n提示: %d/%d 条会话只返回 session id（上游 CLI 限制，不是解析错误）。\n", onlyIDCount, len(items))
				}
				if hydrateCfg.enabled {
					_, _ = fmt.Fprintf(inv.Stdout, "提示: hydrate 已尝试补全 %d 条会话（timeout=%s, maxEvents=%d）。\n", hydratedCount, hydrateCfg.timeout.String(), hydrateCfg.maxEvents)
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

	doctorCmd := &redant.Command{
		Use:      "doctor",
		Short:    "诊断 Copilot 会话配置与依赖",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			resolved, err := resolveCopilotOptions(resolveCopilotInput{
				Model:                 model,
				ReasoningEffort:       reasoningEffort,
				Streaming:             streaming,
				ConfigDir:             configDir,
				SessionWorkingDir:     sessionWorkingDir,
				ProfileName:           profileName,
				ProfileFile:           profileFile,
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
				EnableSkillsTool:      enableSkillsTool,
				EnableInfiniteSession: enableInfiniteSession,
			})
			if err != nil {
				return err
			}

			report := runDoctorChecks(doctorInput{
				GitHubToken:      githubToken,
				UseLoggedInUser:  useLoggedInUser,
				ProfileName:      resolved.ProfileName,
				ProfileFile:      resolved.ProfileFile,
				SkillDirs:        resolved.Advanced.SkillDirectories,
				DisabledSkills:   resolved.Advanced.DisabledSkills,
				AgentName:        strings.TrimSpace(resolved.Advanced.Agent),
				CustomAgentsJSON: resolved.CustomAgentsJSON,
				MCPServersJSON:   resolved.MCPServersJSON,
			})

			for _, line := range report.lines() {
				_, _ = fmt.Fprintln(inv.Stdout, line)
			}

			if report.hasError() {
				return fmt.Errorf("doctor failed: %d error(s), %d warning(s)", report.errorCount(), report.warnCount())
			}
			return nil
		},
	}

	inspectCmd := &redant.Command{
		Use:      "inspect",
		Short:    "查看当前会话将生效的配置摘要",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			resolved, err := resolveCopilotOptions(resolveCopilotInput{
				Model:                 model,
				ReasoningEffort:       reasoningEffort,
				Streaming:             streaming,
				ConfigDir:             configDir,
				SessionWorkingDir:     sessionWorkingDir,
				ProfileName:           profileName,
				ProfileFile:           profileFile,
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
				EnableSkillsTool:      enableSkillsTool,
				EnableInfiniteSession: enableInfiniteSession,
			})
			if err != nil {
				return err
			}

			summary := buildInspectSummary(inspectInput{
				Model:                resolved.Model,
				ReasoningEffort:      resolved.ReasoningEffort,
				Streaming:            resolved.Streaming,
				ConfigDir:            resolved.ConfigDir,
				SessionWorkingDir:    resolved.SessionWorkingDir,
				UseLoggedInUser:      useLoggedInUser,
				HasGitHubToken:       strings.TrimSpace(githubToken) != "",
				EnableDemoEchoTool:   resolved.EnableDemoEchoTool,
				EnableInfiniteSesion: resolved.EnableInfiniteSession,
				ProfileName:          resolved.ProfileName,
				ProfileFile:          resolved.ProfileFile,
				Advanced:             resolved.Advanced,
				CustomAgentsJSON:     resolved.CustomAgentsJSON,
				MCPServersJSON:       resolved.MCPServersJSON,
			})

			payload, err := json.MarshalIndent(summary, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal inspect summary: %w", err)
			}
			_, _ = fmt.Fprintln(inv.Stdout, string(payload))
			return nil
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

	skillsCmd := newSkillsCmd(&profileName, &profileFile, &skillDirs)

	rootCmd.Children = []*redant.Command{chatCmd, resumeCmd, sessionsCmd, statusCmd, modelsCmd, doctorCmd, inspectCmd, skillsCmd, interactiveDemoCmd}
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
	EnableSkillsTool      bool
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

type copilotProfile struct {
	Model                 string   `json:"model"`
	ReasoningEffort       string   `json:"reasoningEffort"`
	Streaming             *bool    `json:"streaming"`
	ConfigDir             string   `json:"configDir"`
	SessionWorkingDir     string   `json:"workingDirectory"`
	SystemMessageMode     string   `json:"systemMessageMode"`
	SystemMessage         string   `json:"systemMessage"`
	SystemSectionsJSON    string   `json:"systemSectionsJSON"`
	SkillDirs             []string `json:"skillDirs"`
	DisabledSkills        []string `json:"disabledSkills"`
	AvailableTools        []string `json:"availableTools"`
	ExcludedTools         []string `json:"excludedTools"`
	MCPServersJSON        string   `json:"mcpServersJSON"`
	CustomAgentsJSON      string   `json:"customAgentsJSON"`
	AgentName             string   `json:"agent"`
	CustomToolsJSON       string   `json:"customToolsJSON"`
	EnableDemoEchoTool    *bool    `json:"enableDemoEchoTool"`
	EnableSkillsTool      *bool    `json:"enableSkillsTool"`
	EnableInfiniteSession *bool    `json:"enableInfiniteSession"`
}

type copilotProfileFile struct {
	Profiles map[string]copilotProfile `json:"profiles"`
}

type resolveCopilotInput struct {
	Model                 string
	ReasoningEffort       string
	Streaming             bool
	ConfigDir             string
	SessionWorkingDir     string
	ProfileName           string
	ProfileFile           string
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
	EnableSkillsTool      bool
	EnableInfiniteSession bool
}

type resolvedCopilotOptions struct {
	Model                 string
	ReasoningEffort       string
	Streaming             bool
	ConfigDir             string
	SessionWorkingDir     string
	ProfileName           string
	ProfileFile           string
	CustomAgentsJSON      string
	MCPServersJSON        string
	EnableDemoEchoTool    bool
	EnableSkillsTool      bool
	EnableInfiniteSession bool
	Advanced              *advancedConfig
}

func resolveCopilotOptions(in resolveCopilotInput) (*resolvedCopilotOptions, error) {
	pf, err := loadCopilotProfile(strings.TrimSpace(in.ProfileFile), strings.TrimSpace(in.ProfileName))
	if err != nil {
		return nil, err
	}

	if pf != nil {
		if strings.TrimSpace(in.Model) == "" || strings.TrimSpace(in.Model) == "gpt-5" {
			if strings.TrimSpace(pf.Model) != "" {
				in.Model = strings.TrimSpace(pf.Model)
			}
		}
		if strings.TrimSpace(in.ReasoningEffort) == "" && strings.TrimSpace(pf.ReasoningEffort) != "" {
			in.ReasoningEffort = strings.TrimSpace(pf.ReasoningEffort)
		}
		if strings.TrimSpace(in.ConfigDir) == "" && strings.TrimSpace(pf.ConfigDir) != "" {
			in.ConfigDir = strings.TrimSpace(pf.ConfigDir)
		}
		if strings.TrimSpace(in.SessionWorkingDir) == "" && strings.TrimSpace(pf.SessionWorkingDir) != "" {
			in.SessionWorkingDir = strings.TrimSpace(pf.SessionWorkingDir)
		}
		if (strings.TrimSpace(in.SystemMessageMode) == "" || strings.EqualFold(strings.TrimSpace(in.SystemMessageMode), "append")) && strings.TrimSpace(pf.SystemMessageMode) != "" {
			in.SystemMessageMode = strings.TrimSpace(pf.SystemMessageMode)
		}
		if strings.TrimSpace(in.SystemMessage) == "" && strings.TrimSpace(pf.SystemMessage) != "" {
			in.SystemMessage = strings.TrimSpace(pf.SystemMessage)
		}
		if strings.TrimSpace(in.SystemSectionsJSON) == "" && strings.TrimSpace(pf.SystemSectionsJSON) != "" {
			in.SystemSectionsJSON = strings.TrimSpace(pf.SystemSectionsJSON)
		}
		if len(in.SkillDirs) == 0 && len(pf.SkillDirs) > 0 {
			in.SkillDirs = append([]string(nil), pf.SkillDirs...)
		}
		if len(in.DisabledSkills) == 0 && len(pf.DisabledSkills) > 0 {
			in.DisabledSkills = append([]string(nil), pf.DisabledSkills...)
		}
		if len(in.AvailableTools) == 0 && len(pf.AvailableTools) > 0 {
			in.AvailableTools = append([]string(nil), pf.AvailableTools...)
		}
		if len(in.ExcludedTools) == 0 && len(pf.ExcludedTools) > 0 {
			in.ExcludedTools = append([]string(nil), pf.ExcludedTools...)
		}
		if strings.TrimSpace(in.MCPServersJSON) == "" && strings.TrimSpace(pf.MCPServersJSON) != "" {
			in.MCPServersJSON = strings.TrimSpace(pf.MCPServersJSON)
		}
		if strings.TrimSpace(in.CustomAgentsJSON) == "" && strings.TrimSpace(pf.CustomAgentsJSON) != "" {
			in.CustomAgentsJSON = strings.TrimSpace(pf.CustomAgentsJSON)
		}
		if strings.TrimSpace(in.AgentName) == "" && strings.TrimSpace(pf.AgentName) != "" {
			in.AgentName = strings.TrimSpace(pf.AgentName)
		}
		if strings.TrimSpace(in.CustomToolsJSON) == "" && strings.TrimSpace(pf.CustomToolsJSON) != "" {
			in.CustomToolsJSON = strings.TrimSpace(pf.CustomToolsJSON)
		}
		if !in.Streaming && pf.Streaming != nil {
			in.Streaming = *pf.Streaming
		}
		if !in.EnableDemoEchoTool && pf.EnableDemoEchoTool != nil {
			in.EnableDemoEchoTool = *pf.EnableDemoEchoTool
		}
		if !in.EnableSkillsTool && pf.EnableSkillsTool != nil {
			in.EnableSkillsTool = *pf.EnableSkillsTool
		}
		if in.EnableInfiniteSession && pf.EnableInfiniteSession != nil {
			in.EnableInfiniteSession = *pf.EnableInfiniteSession
		}
	}

	adv, err := buildAdvancedConfig(advancedConfigInput{
		SystemMessageMode:     in.SystemMessageMode,
		SystemMessage:         in.SystemMessage,
		SystemSectionsJSON:    in.SystemSectionsJSON,
		SkillDirs:             in.SkillDirs,
		DisabledSkills:        in.DisabledSkills,
		AvailableTools:        in.AvailableTools,
		ExcludedTools:         in.ExcludedTools,
		MCPServersJSON:        in.MCPServersJSON,
		CustomAgentsJSON:      in.CustomAgentsJSON,
		AgentName:             in.AgentName,
		CustomToolsJSON:       in.CustomToolsJSON,
		EnableDemoEchoTool:    in.EnableDemoEchoTool,
		EnableSkillsTool:      in.EnableSkillsTool,
		EnableInfiniteSession: in.EnableInfiniteSession,
	})
	if err != nil {
		return nil, err
	}

	return &resolvedCopilotOptions{
		Model:                 strings.TrimSpace(in.Model),
		ReasoningEffort:       strings.TrimSpace(in.ReasoningEffort),
		Streaming:             in.Streaming,
		ConfigDir:             strings.TrimSpace(in.ConfigDir),
		SessionWorkingDir:     strings.TrimSpace(in.SessionWorkingDir),
		ProfileName:           strings.TrimSpace(in.ProfileName),
		ProfileFile:           strings.TrimSpace(in.ProfileFile),
		CustomAgentsJSON:      strings.TrimSpace(in.CustomAgentsJSON),
		MCPServersJSON:        strings.TrimSpace(in.MCPServersJSON),
		EnableDemoEchoTool:    in.EnableDemoEchoTool,
		EnableSkillsTool:      in.EnableSkillsTool,
		EnableInfiniteSession: in.EnableInfiniteSession,
		Advanced:              adv,
	}, nil
}

func loadCopilotProfile(profileFile, profileName string) (*copilotProfile, error) {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return nil, nil
	}
	profileFile = strings.TrimSpace(profileFile)
	if profileFile == "" {
		return nil, fmt.Errorf("--profile-file 不能为空")
	}

	content, err := os.ReadFile(profileFile)
	if err != nil {
		return nil, fmt.Errorf("read profile file(%s): %w", profileFile, err)
	}

	var cfg copilotProfileFile
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("invalid profile file json(%s): %w", profileFile, err)
	}
	if len(cfg.Profiles) == 0 {
		return nil, fmt.Errorf("profile file(%s) 中未找到 profiles", profileFile)
	}

	p, ok := cfg.Profiles[profileName]
	if !ok {
		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("profile %q 不存在，available=%s", profileName, strings.Join(names, ","))
	}
	return &p, nil
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
	if in.EnableSkillsTool {
		tools = append(tools, buildSkillsTool(in.SkillDirs))
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

func buildSkillsTool(skillDirs []string) copilot.Tool {
	defaultDirs := compactStringSlice(skillDirs)
	const schemaVersion = "skills_tool/v1"
	return copilot.Tool{
		Name:           "skills_tool",
		Description:    "Manage local skills with stable descriptor output: list/get/query",
		SkipPermission: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"list", "get", "query"}, "description": "tool action"},
				"name":   map[string]any{"type": "string", "description": "skill name for get/query"},
				"h2":     map[string]any{"type": "string", "description": "h2 title for query"},
				"h3":     map[string]any{"type": "string", "description": "h3 title for query"},
				"dirs": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "override skill dirs",
				},
			},
			"required": []string{"action"},
		},
		Handler: func(invocation copilot.ToolInvocation) (copilot.ToolResult, error) {
			svc := skillsmodule.NewLocalManager()
			action := ""
			name := ""
			h2 := ""
			h3 := ""
			dirs := append([]string(nil), defaultDirs...)

			if m, ok := invocation.Arguments.(map[string]any); ok {
				action = strings.ToLower(strings.TrimSpace(fmt.Sprint(m["action"])))
				name = strings.TrimSpace(fmt.Sprint(m["name"]))
				h2 = strings.TrimSpace(fmt.Sprint(m["h2"]))
				h3 = strings.TrimSpace(fmt.Sprint(m["h3"]))
				if rawDirs, ok := m["dirs"].([]any); ok && len(rawDirs) > 0 {
					override := make([]string, 0, len(rawDirs))
					for _, d := range rawDirs {
						override = append(override, strings.TrimSpace(fmt.Sprint(d)))
					}
					dirs = svc.CompactStringSlice(override)
				}
			}

			if len(dirs) == 0 {
				dirs = svc.ExistingDirs([]string{"./skills", "./.copilot/skills"})
			}

			entries, warns := svc.Discover(dirs)
			response := map[string]any{"schemaVersion": schemaVersion, "action": action, "dirs": dirs, "warnings": warns}

			switch action {
			case "list":
				descriptors := make([]map[string]any, 0, len(entries))
				for _, entry := range entries {
					descriptors = append(descriptors, buildSkillDescriptor(entry, false))
				}
				response["skills"] = descriptors
				response["skillsLegacy"] = entries
			case "get":
				if name == "" {
					return errorToolResult("skills_tool get requires name"), nil
				}
				target, err := svc.FindByName(entries, name)
				if err != nil {
					return errorToolResult(err.Error()), nil
				}
				content, err := svc.ReadSkill(target.Path)
				if err != nil {
					return errorToolResult(err.Error()), nil
				}
				response["skill"] = buildSkillDescriptor(target, true)
				response["skillLegacy"] = target
				response["content"] = content
			case "query":
				if name == "" || h2 == "" {
					return errorToolResult("skills_tool query requires name and h2"), nil
				}
				target, err := svc.FindByName(entries, name)
				if err != nil {
					return errorToolResult(err.Error()), nil
				}
				content, err := svc.ReadSkill(target.Path)
				if err != nil {
					return errorToolResult(err.Error()), nil
				}
				parsed, err := svc.ParseContent(content, target.Name)
				if err != nil {
					return errorToolResult(err.Error()), nil
				}
				headings := []string{h2}
				if strings.TrimSpace(h3) != "" {
					headings = append(headings, h3)
				}
				sectionText, ok := svc.FindSectionContent(parsed.Sections, headings...)
				response["skill"] = buildSkillDescriptor(target, true)
				response["skillLegacy"] = target
				response["headings"] = headings
				response["found"] = ok
				response["sectionContent"] = sectionText
			default:
				return errorToolResult("unsupported action, use list|get|query"), nil
			}

			payload, err := json.Marshal(response)
			if err != nil {
				return errorToolResult(err.Error()), nil
			}
			return copilot.ToolResult{TextResultForLLM: string(payload), ResultType: "success", SessionLog: "skills_tool executed"}, nil
		},
	}
}

func buildSkillDescriptor(entry skillsmodule.Entry, includeSections bool) map[string]any {
	kind := strings.TrimSpace(entry.Kind)
	if kind == "" {
		kind = "local"
	}
	slug := strings.TrimSpace(entry.Slug)
	if slug == "" {
		slug = strings.TrimSpace(entry.Name)
	}
	displayName := strings.TrimSpace(entry.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(entry.Title)
	}
	if displayName == "" {
		displayName = strings.TrimSpace(entry.Name)
	}
	summary := strings.TrimSpace(entry.Summary)
	if summary == "" {
		summary = strings.TrimSpace(entry.Description)
	}

	d := map[string]any{
		"id":          strings.TrimSpace(entry.ID),
		"kind":        kind,
		"namespace":   strings.TrimSpace(entry.Namespace),
		"name":        strings.TrimSpace(entry.Name),
		"slug":        slug,
		"displayName": displayName,
		"summary":     summary,
		"description": strings.TrimSpace(entry.Description),
		"version":     strings.TrimSpace(entry.Version),
		"tags":        entry.Tags,
		"useWhen":     entry.UseWhen,
		"tools":       entry.Tools,
		"path":        strings.TrimSpace(entry.Path),
		"dir":         strings.TrimSpace(entry.Dir),
		"source":      strings.TrimSpace(entry.Source),
		"headings": map[string]any{
			"h2": entry.H2,
			"h3": entry.H3,
		},
		"metadata": entry.Metadata,
	}
	if includeSections {
		d["sections"] = entry.Sections
	}
	return d
}

func errorToolResult(msg string) copilot.ToolResult {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		msg = "unknown error"
	}
	return copilot.ToolResult{TextResultForLLM: msg, ResultType: "error", SessionLog: "tool error: " + msg}
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

type doctorInput struct {
	GitHubToken      string
	UseLoggedInUser  bool
	ProfileName      string
	ProfileFile      string
	SkillDirs        []string
	DisabledSkills   []string
	AgentName        string
	CustomAgentsJSON string
	MCPServersJSON   string
}

type doctorFinding struct {
	Level string
	Text  string
}

type doctorReport struct {
	findings []doctorFinding
}

func runDoctorChecks(in doctorInput) doctorReport {
	report := doctorReport{}

	if strings.TrimSpace(in.ProfileName) != "" {
		report.add("ok", fmt.Sprintf("使用 profile=%s (file=%s)", strings.TrimSpace(in.ProfileName), withDefault(strings.TrimSpace(in.ProfileFile), "(empty)")))
	}

	if strings.TrimSpace(in.GitHubToken) == "" && !in.UseLoggedInUser {
		report.add("error", "未提供 --copilot-token/GITHUB_TOKEN，且 --copilot-use-logged-in-user=false，无法认证")
	} else if strings.TrimSpace(in.GitHubToken) == "" && in.UseLoggedInUser {
		report.add("warn", "未提供 token，将依赖已登录用户身份")
	} else {
		report.add("ok", "认证配置可用（检测到 token 或已登录用户模式）")
	}

	if len(in.SkillDirs) == 0 {
		report.add("warn", "未配置 --skill-dirs，skills 不会从外部目录加载")
	} else {
		for _, dir := range in.SkillDirs {
			if err := checkSkillDir(dir); err != nil {
				report.add("error", err.Error())
				continue
			}
			report.add("ok", "skills 目录可用: "+dir)
		}
	}

	if len(in.DisabledSkills) > 0 {
		report.add("ok", fmt.Sprintf("禁用技能数: %d", len(in.DisabledSkills)))
	}

	agentNames := extractCustomAgentNamesFromJSON(in.CustomAgentsJSON)
	agentNameSet := make(map[string]struct{}, len(agentNames))
	for _, item := range agentNames {
		agentNameSet[item] = struct{}{}
	}
	if in.AgentName != "" {
		if len(agentNameSet) == 0 {
			report.add("warn", "指定了 --agent，但未通过 --custom-agents-json 提供可选 agent 列表")
		} else if _, ok := agentNameSet[in.AgentName]; !ok {
			report.add("error", fmt.Sprintf("--agent=%q 不在 custom-agents 列表中", in.AgentName))
		} else {
			report.add("ok", "激活 agent 匹配成功: "+in.AgentName)
		}
	} else if len(agentNameSet) > 0 {
		report.add("warn", "已配置 custom-agents，但未指定 --agent 激活项")
	}

	mcpCommands := extractMCPServerCommands(in.MCPServersJSON)
	if len(mcpCommands) == 0 {
		report.add("warn", "未配置 --mcp-servers-json")
	} else {
		for _, item := range mcpCommands {
			if item.Command == "" {
				report.add("warn", fmt.Sprintf("MCP server=%q 未配置 command", item.Name))
				continue
			}
			if _, err := exec.LookPath(item.Command); err != nil {
				report.add("error", fmt.Sprintf("MCP server=%q command=%q 不可执行: %v", item.Name, item.Command, err))
				continue
			}
			report.add("ok", fmt.Sprintf("MCP server=%q command 可执行: %s", item.Name, item.Command))
		}
	}

	return report
}

func (r *doctorReport) add(level, text string) {
	r.findings = append(r.findings, doctorFinding{Level: strings.TrimSpace(level), Text: strings.TrimSpace(text)})
}

func (r doctorReport) lines() []string {
	out := make([]string, 0, len(r.findings)+1)
	out = append(out, "Copilot doctor report")
	for _, item := range r.findings {
		prefix := "[INFO]"
		switch strings.ToLower(strings.TrimSpace(item.Level)) {
		case "ok":
			prefix = "[ OK ]"
		case "warn":
			prefix = "[WARN]"
		case "error":
			prefix = "[ERR ]"
		}
		out = append(out, fmt.Sprintf("%s %s", prefix, item.Text))
	}
	out = append(out, fmt.Sprintf("summary: errors=%d warnings=%d", r.errorCount(), r.warnCount()))
	return out
}

func (r doctorReport) errorCount() int {
	n := 0
	for _, item := range r.findings {
		if strings.EqualFold(strings.TrimSpace(item.Level), "error") {
			n++
		}
	}
	return n
}

func (r doctorReport) warnCount() int {
	n := 0
	for _, item := range r.findings {
		if strings.EqualFold(strings.TrimSpace(item.Level), "warn") {
			n++
		}
	}
	return n
}

func (r doctorReport) hasError() bool {
	return r.errorCount() > 0
}

func checkSkillDir(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("skills 目录为空")
	}
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("skills 目录不可用(%s): %w", path, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("skills 路径不是目录(%s)", path)
	}
	return nil
}

type namedCommand struct {
	Name    string
	Command string
}

func extractMCPServerCommands(raw string) []namedCommand {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	out := make([]namedCommand, 0, len(payload))
	for name, cfg := range payload {
		cmd := ""
		if cfg != nil {
			if v, ok := cfg["command"]; ok {
				cmd = strings.TrimSpace(fmt.Sprint(v))
				if cmd == "<nil>" {
					cmd = ""
				}
			}
		}
		out = append(out, namedCommand{Name: strings.TrimSpace(name), Command: cmd})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func extractCustomAgentNamesFromJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var payload []map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	out := make([]string, 0, len(payload))
	for _, item := range payload {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name == "" || name == "<nil>" {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

type inspectInput struct {
	Model                string
	ReasoningEffort      string
	Streaming            bool
	ConfigDir            string
	SessionWorkingDir    string
	UseLoggedInUser      bool
	HasGitHubToken       bool
	EnableDemoEchoTool   bool
	EnableInfiniteSesion bool
	ProfileName          string
	ProfileFile          string
	Advanced             *advancedConfig
	CustomAgentsJSON     string
	MCPServersJSON       string
}

func buildInspectSummary(in inspectInput) map[string]any {
	systemMessage := map[string]any{"enabled": false}
	if in.Advanced != nil && in.Advanced.SystemMessage != nil {
		systemMessage["enabled"] = true
		systemMessage["mode"] = strings.TrimSpace(in.Advanced.SystemMessage.Mode)
		systemMessage["hasContent"] = strings.TrimSpace(in.Advanced.SystemMessage.Content) != ""
		systemMessage["sectionOverrideCount"] = len(in.Advanced.SystemMessage.Sections)
	}

	mcpServers := extractMCPServerCommands(in.MCPServersJSON)
	mcpNames := make([]string, 0, len(mcpServers))
	for _, item := range mcpServers {
		mcpNames = append(mcpNames, item.Name)
	}

	toolNames := make([]string, 0)
	if in.Advanced != nil {
		toolNames = extractToolNames(in.Advanced.Tools)
	}

	return map[string]any{
		"model":                  withDefault(strings.TrimSpace(in.Model), "gpt-5"),
		"reasoningEffort":        strings.TrimSpace(in.ReasoningEffort),
		"profile":                map[string]any{"name": strings.TrimSpace(in.ProfileName), "file": strings.TrimSpace(in.ProfileFile)},
		"streaming":              in.Streaming,
		"configDir":              strings.TrimSpace(in.ConfigDir),
		"workingDirectory":       strings.TrimSpace(in.SessionWorkingDir),
		"auth":                   map[string]any{"useLoggedInUser": in.UseLoggedInUser, "hasGitHubToken": in.HasGitHubToken},
		"systemMessage":          systemMessage,
		"skillDirectories":       safeSlice(in.Advanced, func(a *advancedConfig) []string { return a.SkillDirectories }),
		"disabledSkills":         safeSlice(in.Advanced, func(a *advancedConfig) []string { return a.DisabledSkills }),
		"availableTools":         safeSlice(in.Advanced, func(a *advancedConfig) []string { return a.AvailableTools }),
		"excludedTools":          safeSlice(in.Advanced, func(a *advancedConfig) []string { return a.ExcludedTools }),
		"customToolNames":        toolNames,
		"enableDemoEchoTool":     in.EnableDemoEchoTool,
		"customAgentNames":       extractCustomAgentNamesFromJSON(in.CustomAgentsJSON),
		"activeAgent":            safeString(in.Advanced, func(a *advancedConfig) string { return a.Agent }),
		"mcpServers":             mcpNames,
		"enableInfiniteSessions": in.EnableInfiniteSesion,
	}
}

func extractToolNames(tools []copilot.Tool) []string {
	if len(tools) == 0 {
		return nil
	}
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func safeSlice(adv *advancedConfig, get func(*advancedConfig) []string) []string {
	if adv == nil {
		return nil
	}
	return get(adv)
}

func safeString(adv *advancedConfig, get func(*advancedConfig) string) string {
	if adv == nil {
		return ""
	}
	return strings.TrimSpace(get(adv))
}
