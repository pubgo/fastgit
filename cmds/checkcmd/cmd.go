package checkcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pubgo/redant"
)

const hookMarker = "# fastgit-managed"

// New creates the check command group.
func New() *redant.Command {
	root := &redant.Command{
		Use:   "check",
		Short: "统一质量门禁：fmt / lint / test / secret scan",
		Long:  "在 commit/push 前一键执行质量检查。支持 dry-run、staged-only、自动修复与钩子安装。",
	}

	root.Children = []*redant.Command{
		newRunCommand(),
		newConfigCommand(),
		newHookCommand(),
	}

	return root
}

func newRunCommand() *redant.Command {
	var (
		stagedOnly bool
		fix        bool
		dryRun     bool
	)

	return &redant.Command{
		Use:   "run",
		Short: "执行质量检查流水线",
		Options: redant.OptionSet{
			{Flag: "staged-only", Description: "仅检查 staged 文件（fmt 会限定到 staged .go）", Value: redant.BoolOf(&stagedOnly)},
			{Flag: "fix", Description: "对可修复项先修复（如 gofmt -w）", Value: redant.BoolOf(&fix)},
			{Flag: "dry-run", Description: "只输出将执行的步骤，不修改仓库", Value: redant.BoolOf(&dryRun)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}

			cfg := LoadConfig(repoRoot)
			opts := RunOptions{
				StagedOnly: stagedOnly,
				Fix:        fix,
				DryRun:     dryRun,
				RepoRoot:   repoRoot,
			}
			stagedFiles, _ := listStagedFiles(repoRoot)
			if len(stagedFiles) > 0 {
				warnSensitiveStaged(inv, repoRoot, stagedFiles)
			}
			results, err := Run(ctx, cfg, opts)
			printResults(inv, results, dryRun, err)
			return err
		},
	}
}

func newConfigCommand() *redant.Command {
	return &redant.Command{
		Use:   "config",
		Short: "展示当前门禁配置",
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}

			cfg := LoadConfig(repoRoot)
			_, _ = fmt.Fprintln(inv.Stdout, "check pipeline:")
			for _, step := range cfg.Steps {
				cmd := step.Command
				if cmd == "" {
					cmd = "(not configured)"
				}
				optional := ""
				if step.Optional {
					optional = " [optional]"
				}
				_, _ = fmt.Fprintf(inv.Stdout, "  - %s%s: %s\n", step.Name, optional, cmd)
				if step.Fixable && step.FixCommand != "" {
					_, _ = fmt.Fprintf(inv.Stdout, "      fix: %s\n", step.FixCommand)
				}
			}
			_, _ = fmt.Fprintf(inv.Stdout, "repo: %s\n", repoRoot)
			_, _ = fmt.Fprintln(inv.Stdout, "config file: .fastgit/check.yaml")
			return nil
		},
	}
}

func newHookCommand() *redant.Command {
	var force bool

	hook := &redant.Command{
		Use:   "hook",
		Short: "安装或卸载 git 钩子（pre-commit / pre-push）",
	}

	hook.Children = []*redant.Command{
		{
			Use:   "install",
			Short: "安装 pre-commit 与 pre-push 钩子",
			Long:  "pre-commit 运行 `fastgit check run --staged-only`；pre-push 运行 `fastgit check run`。与 lefthook 等工具共存时，请勿覆盖非 fastgit 管理的钩子。",
			Options: redant.OptionSet{
				{Flag: "force", Description: "覆盖已有非 fastgit 钩子", Value: redant.BoolOf(&force)},
			},
			Handler: func(ctx context.Context, inv *redant.Invocation) error {
				_ = ctx
				return installHooks(inv, force)
			},
		},
		{
			Use:   "uninstall",
			Short: "移除 fastgit 管理的 pre-commit / pre-push 钩子",
			Handler: func(ctx context.Context, inv *redant.Invocation) error {
				_ = ctx
				return uninstallHooks(inv)
			},
		},
	}

	return hook
}

func printResults(inv *redant.Invocation, results []StepResult, dryRun bool, runErr error) {
	for _, r := range results {
		if r.Output != "" {
			_, _ = fmt.Fprintln(inv.Stdout, r.Output)
		}
		if r.Skipped && r.Reason != "" {
			_, _ = fmt.Fprintf(inv.Stdout, "skip %s: %s\n", r.Step.Name, r.Reason)
		}
	}
	if dryRun {
		_, _ = fmt.Fprintln(inv.Stdout, "dry-run complete (no changes made)")
		return
	}
	if runErr != nil {
		_, _ = fmt.Fprintln(inv.Stderr, FormatFailureSummary(results, runErr))
		return
	}
	_, _ = fmt.Fprintln(inv.Stdout, "check passed")
}
