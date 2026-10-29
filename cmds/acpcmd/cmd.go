package acpcmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/pubgo/fastgit/configs"
	"github.com/pubgo/fastgit/pkg/acpclient"
	"github.com/pubgo/fastgit/pkg/acpconfig"
	"github.com/pubgo/fastgit/pkg/acpui"
	"github.com/pubgo/redant"
	"github.com/samber/lo"
)

type flagOptions struct {
	command  string
	args     string
	env      string
	prompt   string
	timeout  string
	agent    string
	permit   string
	clientFS bool
	clear    bool
}

// permissionHandler 按策略名构造权限回调: ask|auto|deny。
func permissionHandler(mode string) (acpclient.PermissionHandler, error) {
	return acpui.Permission(mode)
}

func runTimeout(seconds string) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(seconds)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return acpclient.DefaultTimeout
}

func parseAgentFlags(flags *flagOptions) (acpconfig.AgentConfig, error) {
	def := acpconfig.AgentConfig{
		Command: strings.TrimSpace(flags.command),
	}
	if a := strings.TrimSpace(flags.args); a != "" {
		def.Args = strings.Fields(a)
	}
	if e := strings.TrimSpace(flags.env); e != "" {
		def.Env = map[string]string{}
		for _, item := range strings.Split(e, ",") {
			name, value, ok := strings.Cut(item, "=")
			if !ok {
				return def, fmt.Errorf("invalid env item %q, want NAME=value", item)
			}
			def.Env[strings.TrimSpace(name)] = value
		}
	}
	return def, nil
}

func New() *redant.Command {
	var flags = new(flagOptions)

	return &redant.Command{
		Use:   "acp",
		Short: "manage ACP agents (agent client protocol) used by commit/review ai generation",
		Children: []*redant.Command{
			{
				Use:   "list",
				Short: "list configured and built-in acp agents",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					cfg, err := acpconfig.Load()
					if err != nil {
						return err
					}

					fmt.Println("config:", configs.GetConfigPath())
					fmt.Println()
					fmt.Printf("%-12s %-6s %-8s %s\n", "NAME", "SOURCE", "STATUS", "COMMAND")
					for _, name := range lo.Keys(cfg.Agents) {
						def := cfg.Agents[name]
						fmt.Printf("%-12s %-6s %-8s %s\n", name, "config", statusMark(def.Command), commandLine(def))
					}
					for _, name := range []string{"claude", "codex", "gemini", "opencode"} {
						if _, ok := cfg.Agents[name]; ok {
							continue
						}
						def := acpconfig.Catalog[name]
						fmt.Printf("%-12s %-6s %-8s %s\n", name, "catalog", statusMark(def.Command), commandLine(def))
					}
					fmt.Println()
					def := cfg.Default
					if def == "" {
						def = "(auto-detect)"
					}
					fmt.Println("default:", def, " env override:", acpconfig.EnvAgentVar)
					return nil
				},
			},
			{
				Use:   "add",
				Short: "add or update an acp agent: add <name> [catalog name] [--command X --args \"a b\" --env K=V,K2=V2]",
				Options: []redant.Option{
					{Flag: "command", Description: "executable to spawn the acp agent", Value: redant.StringOf(&flags.command)},
					{Flag: "args", Description: "arguments, space separated", Value: redant.StringOf(&flags.args)},
					{Flag: "env", Description: "extra env, comma separated NAME=value pairs", Value: redant.StringOf(&flags.env)},
				},
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					args := i.Command.Args
					if len(args) == 0 {
						return fmt.Errorf("usage: ggc acp add <name> [--command X --args \"a b\"]")
					}
					name := strings.TrimSpace(args[0].Value.String())

					def, err := parseAgentFlags(flags)
					if err != nil {
						return err
					}
					if def.Command == "" {
						catalogDef, ok := acpconfig.Catalog[name]
						if !ok {
							return fmt.Errorf("agent %q is not a built-in catalog name; pass --command (catalog: claude, codex, gemini, opencode)", name)
						}
						def = catalogDef
					}

					if err := acpconfig.AddAgent(name, def); err != nil {
						return err
					}
					fmt.Printf("acp agent %q saved: %s\n", name, commandLine(def))
					return nil
				},
			},
			{
				Use:   "remove",
				Short: "remove an acp agent from config",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					args := i.Command.Args
					if len(args) == 0 {
						return fmt.Errorf("usage: ggc acp remove <name>")
					}
					name := strings.TrimSpace(args[0].Value.String())
					if err := acpconfig.RemoveAgent(name); err != nil {
						return err
					}
					fmt.Printf("acp agent %q removed\n", name)
					return nil
				},
			},
			{
				Use:   "set-default",
				Short: "set the default acp agent (name, or --clear to unset)",
				Options: []redant.Option{
					{Flag: "clear", Description: "unset acp.default", Value: redant.BoolOf(&flags.clear)},
				},
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					name := ""
					if !flags.clear {
						args := i.Command.Args
						if len(args) == 0 {
							return fmt.Errorf("usage: ggc acp set-default <name> | --clear")
						}
						name = strings.TrimSpace(args[0].Value.String())
					}
					if err := acpconfig.SetDefault(name); err != nil {
						return err
					}
					if name == "" {
						fmt.Println("acp.default cleared (auto mode)")
					} else {
						fmt.Printf("acp default agent set to %q\n", name)
					}
					return nil
				},
			},
			{
				Use:   "run",
				Short: "run one agent-mode prompt through an acp agent: run <prompt...> [--agent name] [--permit ask|auto|deny] [--timeout s] [--client-fs]",
				Options: []redant.Option{
					{Flag: "agent", Description: "acp agent 名(默认按 acp.default/自动探测)", Value: redant.StringOf(&flags.agent)},
					{Flag: "permit", Description: "权限策略 ask|auto|deny", Value: redant.StringOf(&flags.permit), Default: "ask"},
					{Flag: "timeout", Description: "整轮超时秒数", Value: redant.StringOf(&flags.timeout), Default: "300"},
					{Flag: "client-fs", Description: "声明并处理 client 侧 fs 读写(沙箱限定当前目录)", Value: redant.BoolOf(&flags.clientFS)},
				},
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					parts := make([]string, 0, len(i.Command.Args))
					for _, arg := range i.Command.Args {
						parts = append(parts, arg.Value.String())
					}
					prompt := strings.TrimSpace(strings.Join(parts, " "))
					if prompt == "" {
						return fmt.Errorf("usage: ggc acp run <prompt...>")
					}

					cfg, err := acpconfig.Load()
					if err != nil {
						return err
					}
					agent, err := cfg.Resolve(strings.TrimSpace(flags.agent))
					if err != nil {
						return err
					}
					onPermission, err := permissionHandler(flags.permit)
					if err != nil {
						return err
					}

					fmt.Printf("→ acp run agent=%q permit=%s timeout=%s\n", agent.Name, strings.ToLower(strings.TrimSpace(flags.permit)), runTimeout(flags.timeout))
					result, err := acpclient.Run(ctx, agent, acpclient.Options{
						Prompt:       prompt,
						Timeout:      runTimeout(flags.timeout),
						OnPermission: onPermission,
						ClientFS:     flags.clientFS,
						OnUpdate: func(update acp.SessionUpdate) error {
							if line := acpclient.UpdateLine(update); line != "" {
								fmt.Fprintln(os.Stderr, line)
							}
							return nil
						},
					})
					if err != nil {
						return err
					}
					fmt.Println(result.Text)
					return nil
				},
			},
			{
				Use:   "test",
				Short: "spawn the agent, initialize an ACP session and run a ping prompt",
				Options: []redant.Option{
					{Flag: "prompt", Description: "prompt for the test turn", Value: redant.StringOf(&flags.prompt), Default: "请只回复两个字:成功"},
					{Flag: "timeout", Description: "round-trip timeout in seconds", Value: redant.StringOf(&flags.timeout), Default: "120"},
				},
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					name := ""
					if args := i.Command.Args; len(args) > 0 {
						name = strings.TrimSpace(args[0].Value.String())
					}

					cfg, err := acpconfig.Load()
					if err != nil {
						return err
					}
					agent, err := cfg.Resolve(name)
					if err != nil {
						return err
					}

					timeout := runTimeout(flags.timeout)

					fmt.Printf("→ testing acp agent %q: %s (timeout %s)\n", agent.Name, agent.Command+" "+strings.Join(agent.Args, " "), timeout)
					start := time.Now()
					result, err := acpclient.Run(ctx, agent, acpclient.Options{
						Prompt:  flags.prompt,
						Timeout: timeout,
					})
					if err != nil {
						return err
					}
					fmt.Printf("✅ acp agent %q ok in %s\n", agent.Name, time.Since(start).Round(time.Millisecond))
					fmt.Println("reply:", result.Text)
					return nil
				},
			},
		},
	}
}

func statusMark(command string) string {
	if acpconfig.HasExecutable(command) {
		return "ready"
	}
	return "missing"
}

func commandLine(def acpconfig.AgentConfig) string {
	parts := make([]string, 0, len(def.Args)+1)
	parts = append(parts, def.Command)
	for _, arg := range def.Args {
		if strings.ContainsAny(arg, " \t\"'") {
			arg = fmt.Sprintf("%q", arg)
		}
		parts = append(parts, arg)
	}
	return strings.Join(parts, " ")
}
