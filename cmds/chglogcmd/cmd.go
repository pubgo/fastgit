package chglogcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/fastgit/pkg/copilotperm"
	"github.com/pubgo/redant"
)

// NewCommand creates the changelog command group.
func NewCommand() *redant.Command {
	root := &redant.Command{
		Use:   "changelog",
		Short: "使用 Copilot 工作流维护 .version/changelog",
		Long:  "初始化 changelog 模板、用 Copilot 维护 Unreleased，或执行版本落版。",
	}

	root.Children = []*redant.Command{
		newInitCommand(),
		newDraftCommand(),
		newReleaseCommand(),
	}

	return root
}

func newInitCommand() *redant.Command {
	var (
		repoPath string
		version  string
		force    bool
	)

	return &redant.Command{
		Use:   "init",
		Short: "初始化 .version/changelog 模板",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "目标仓库目录（默认当前目录）", Value: redant.StringOf(&repoPath)},
			{Flag: "version", Description: "初始化版本号（默认 v0.1.0）", Value: redant.StringOf(&version), Default: defaultInitialVersion},
			{Flag: "force", Description: "覆盖已有模板文件", Value: redant.BoolOf(&force), Default: "false"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			repoRoot, err := resolveRepoRoot(strings.TrimSpace(repoPath))
			if err != nil {
				return err
			}

			result, err := ensureChangelogScaffold(repoRoot, scaffoldOptions{
				Version:                strings.TrimSpace(version),
				Force:                  force,
				CreateVersionIfMissing: true,
			})
			if err != nil {
				return err
			}

			for _, file := range result.Created {
				_, _ = fmt.Fprintf(inv.Stdout, "created: %s\n", file)
			}
			for _, file := range result.Updated {
				_, _ = fmt.Fprintf(inv.Stdout, "updated: %s\n", file)
			}
			if len(result.Created) == 0 && len(result.Updated) == 0 {
				_, _ = fmt.Fprintln(inv.Stdout, "changelog scaffold already up to date")
			}
			_, _ = fmt.Fprintf(inv.Stdout, "repo: %s\n", repoRoot)
			return nil
		},
	}
}

func newDraftCommand() *redant.Command {
	var (
		repoPath        string
		baseRef         string
		printPrompt     bool
		enrich          bool
		cliPath         string
		logLevel        string
		githubToken     string
		useLoggedInUser bool
		model           string
		reasoningEffort string
		streaming       bool
		autoUserAnswer  string
		permissionMode  string
	)

	return &redant.Command{
		Use:   "draft",
		Short: "使用 Copilot 根据当前改动更新 Unreleased.md",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "目标仓库目录（默认当前目录）", Value: redant.StringOf(&repoPath)},
			{Flag: "base", Description: "diff 基线（默认自动探测）", Value: redant.StringOf(&baseRef)},
			{Flag: "print-prompt", Description: "只打印最终 prompt，不调用 Copilot", Value: redant.BoolOf(&printPrompt), Default: "false"},
			{Flag: "enrich", Description: "用规则引擎预填 影响范围/验证建议/回滚建议", Value: redant.BoolOf(&enrich), Default: "false"},
			{Flag: "copilot-cli-path", Description: "Copilot CLI 可执行路径（可选）", Value: redant.StringOf(&cliPath)},
			{Flag: "copilot-log-level", Description: "Copilot CLI 日志级别", Value: redant.StringOf(&logLevel), Default: "error"},
			{Flag: "copilot-token", Description: "GitHub Token（可选）", Value: redant.StringOf(&githubToken), Envs: []string{"GITHUB_TOKEN"}},
			{Flag: "copilot-use-logged-in-user", Description: "是否使用已登录用户身份", Value: redant.BoolOf(&useLoggedInUser), Default: "true"},
			{Flag: "model", Description: "会话模型", Value: redant.StringOf(&model), Default: "gpt-5"},
			{Flag: "reasoning-effort", Description: "推理强度(low/medium/high/xhigh)", Value: redant.StringOf(&reasoningEffort), Default: "medium"},
			{Flag: "stream", Description: "启用流式输出", Value: redant.BoolOf(&streaming), Default: "false"},
			{Flag: "auto-user-answer", Description: "ask_user 触发时自动回答内容", Value: redant.StringOf(&autoUserAnswer), Default: "继续执行"},
			{Flag: "permission-mode", Description: "Copilot 权限策略 ask|allow|deny（默认 deny；可被 config/env 覆盖）", Value: redant.StringOf(&permissionMode)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := resolveExistingGitRepo(strings.TrimSpace(repoPath))
			if err != nil {
				return err
			}

			result, err := ensureChangelogScaffold(repoRoot, scaffoldOptions{
				Version:                defaultInitialVersion,
				CreateVersionIfMissing: true,
			})
			if err != nil {
				return err
			}
			for _, file := range result.Created {
				_, _ = fmt.Fprintf(inv.Stdout, "created: %s\n", file)
			}

			prompt, detectedBase, err := buildDraftPrompt(ctx, repoRoot, strings.TrimSpace(baseRef))
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(inv.Stdout, "repo: %s\nbase: %s\n", repoRoot, detectedBase)

			if enrich {
				diffNames, err := gitOutput(ctx, repoRoot, "diff", detectedBase, "--name-only")
				if err != nil {
					return err
				}
				paths := buildPaths(repoRoot)
				meta := DeriveReleaseMeta(diffNames)
				changed, err := ApplyReleaseMetaToUnreleased(paths.UnreleasedFile, meta)
				if err != nil {
					return err
				}
				if changed {
					_, _ = fmt.Fprintln(inv.Stdout, "enrich: updated Unreleased.md meta sections")
				} else {
					_, _ = fmt.Fprintln(inv.Stdout, "enrich: meta sections already filled")
				}
			}

			if printPrompt {
				_, _ = fmt.Fprintln(inv.Stdout, prompt)
				return nil
			}

			mode, err := copilotperm.ResolveMode(strings.TrimSpace(permissionMode), copilotperm.ModeDeny)
			if err != nil {
				return err
			}

			return runDraftWithCopilot(ctx, inv, prompt, draftCopilotOptions{
				CLIPath:         strings.TrimSpace(cliPath),
				LogLevel:        strings.TrimSpace(logLevel),
				WorkingDir:      repoRoot,
				GitHubToken:     strings.TrimSpace(githubToken),
				UseLoggedInUser: useLoggedInUser,
				Model:           strings.TrimSpace(model),
				ReasoningEffort: strings.TrimSpace(reasoningEffort),
				Streaming:       streaming,
				AutoUserAnswer:  strings.TrimSpace(autoUserAnswer),
				PermissionMode:  string(mode),
			})
		},
	}
}

func newReleaseCommand() *redant.Command {
	var (
		repoPath    string
		version     string
		nextVersion string
		bump        string
		dryRun        bool
		skipValidate  bool
		skipBumpCheck bool
	)

	return &redant.Command{
		Use:   "release",
		Short: "将 Unreleased.md 落版为版本文件并重建模板",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "目标仓库目录（默认当前目录）", Value: redant.StringOf(&repoPath)},
			{Flag: "version", Description: "发布版本号（默认读取 .version/VERSION）", Value: redant.StringOf(&version)},
			{Flag: "next-version", Description: "发布后写回 .version/VERSION 的下一个版本号", Value: redant.StringOf(&nextVersion)},
			{Flag: "bump", Description: "自动计算下一个版本号（patch|minor|major）", Value: redant.StringOf(&bump)},
			{Flag: "dry-run", Description: "仅预览将要改动的文件，不写入磁盘", Value: redant.BoolOf(&dryRun), Default: "false"},
			{Flag: "skip-validate", Description: "跳过 Unreleased 完整性校验（影响/验证/回滚）", Value: redant.BoolOf(&skipValidate), Default: "false"},
			{Flag: "skip-bump-check", Description: "跳过 bump 与变更类型一致性校验", Value: redant.BoolOf(&skipBumpCheck), Default: "false"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			repoRoot, err := resolveRepoRoot(strings.TrimSpace(repoPath))
			if err != nil {
				return err
			}

			result, err := releaseChangelog(repoRoot, releaseOptions{
				Version:       strings.TrimSpace(version),
				NextVersion:   strings.TrimSpace(nextVersion),
				Bump:          strings.TrimSpace(bump),
				DryRun:        dryRun,
				SkipValidate:  skipValidate,
				SkipBumpCheck: skipBumpCheck,
			})
			if err != nil {
				return err
			}

			for _, file := range result.CreatedFiles {
				prefix := "created"
				if dryRun {
					prefix = "would create"
				}
				_, _ = fmt.Fprintf(inv.Stdout, "%s: %s\n", prefix, file)
			}
			for _, file := range result.UpdatedFiles {
				prefix := "updated"
				if dryRun {
					prefix = "would update"
				}
				_, _ = fmt.Fprintf(inv.Stdout, "%s: %s\n", prefix, file)
			}
			if result.NextVersion != "" {
				_, _ = fmt.Fprintf(inv.Stdout, "next version: %s\n", result.NextVersion)
			}
			return nil
		},
	}
}
