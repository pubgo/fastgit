package prcmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pubgo/fastgit/pkg/aiprovider"
	"github.com/pubgo/fastgit/pkg/repoconfig"
	"github.com/pubgo/redant"
)

// New creates the pr command group.
func New() *redant.Command {
	root := &redant.Command{
		Use:   "pr",
		Short: "Pull Request 流程：create / status / sync / merge",
		Long:  "在 fastgit 内完成本地改动到 PR 的主要路径。依赖 gh CLI 与 GitHub 远端。",
	}

	root.Children = []*redant.Command{
		newCreateCommand(),
		newStatusCommand(),
		newSyncCommand(),
		newMergeCommand(),
	}

	return root
}

func newCreateCommand() *redant.Command {
	var (
		dryRun     bool
		baseRef    string
		repo       string
		useAI      bool
		aiProvider string
	)

	return &redant.Command{
		Use:   "create",
		Short: "创建 PR（自动生成标题与正文）",
		Options: redant.OptionSet{
			{Flag: "dry-run", Description: "只输出将提交的 PR 标题与正文", Value: redant.BoolOf(&dryRun)},
			{Flag: "base", Description: "目标 base 分支（默认自动探测）", Value: redant.StringOf(&baseRef)},
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
			{Flag: "ai", Description: "使用 AI 润色 PR 标题与正文（失败时保留规则版）", Value: redant.BoolOf(&useAI)},
			{Flag: "ai-provider", Description: "AI 提供方 auto|openai|copilot", Value: redant.StringOf(&aiProvider), Default: "auto"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}

			rc, err := LoadRepoContext(ctx, repoRoot)
			if err != nil {
				return err
			}
			if cfg, cfgErr := repoconfig.Load(repoRoot); cfgErr == nil {
				if err := cfg.ValidateBranch(rc.Branch); err != nil {
					_, _ = fmt.Fprintf(inv.Stdout, "policy warning: %v\n", err)
				}
			}
			if base := strings.TrimSpace(baseRef); base != "" {
				rc.BaseRef, err = detectBaseRef(ctx, repoRoot, base)
				if err != nil {
					return err
				}
			}

			draft, err := BuildDraft(ctx, rc)
			if err != nil {
				return err
			}

			if useAI {
				provider := aiprovider.ResolveProvider(aiProvider, repoRoot)
				enhanced, ok, aiErr := EnhanceDraft(ctx, provider, draft)
				if aiErr != nil {
					_, _ = fmt.Fprintf(inv.Stdout, "ai enhance skipped: %v\n", aiErr)
				} else if ok {
					draft = enhanced
					_, _ = fmt.Fprintln(inv.Stdout, "ai: enhanced PR title and body")
				} else {
					_, _ = fmt.Fprintln(inv.Stdout, "ai: no provider available, using rule-based draft")
				}
			}

			_, _ = fmt.Fprintf(inv.Stdout, "branch: %s\nbase: %s -> %s\n", rc.Branch, rc.BaseRef, draft.Base)
			_, _ = fmt.Fprintf(inv.Stdout, "title: %s\n\n", draft.Title)
			_, _ = fmt.Fprintln(inv.Stdout, draft.Body)

			if dryRun {
				_, _ = fmt.Fprintln(inv.Stdout, "\ndry-run complete (PR not created)")
				return nil
			}

			gh := NewGhClient(repoRoot)
			if err := gh.EnsureAvailable(ctx); err != nil {
				return err
			}
			url, err := gh.CreatePR(ctx, draft)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(inv.Stdout, "created: %s\n", url)
			return nil
		},
	}
}

func newStatusCommand() *redant.Command {
	var (
		dryRun bool
		repo   string
	)

	return &redant.Command{
		Use:   "status",
		Short: "查看当前分支 PR 状态",
		Options: redant.OptionSet{
			{Flag: "dry-run", Description: "只说明将查询的内容，不调用 gh", Value: redant.BoolOf(&dryRun)},
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}

			rc, err := LoadRepoContext(ctx, repoRoot)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(inv.Stdout, "branch: %s\nupstream: %s\n", rc.Branch, rc.Upstream)

			if dryRun {
				_, _ = fmt.Fprintln(inv.Stdout, "dry-run: would run `gh pr view` for current branch")
				return nil
			}

			gh := NewGhClient(repoRoot)
			if err := gh.EnsureAvailable(ctx); err != nil {
				return err
			}
			view, err := gh.ViewPR(ctx)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(inv.Stdout, view)
			return nil
		},
	}
}

func newSyncCommand() *redant.Command {
	var (
		dryRun       bool
		repo         string
		baseRef      string
		updateBody   bool
		useAI        bool
		aiProvider   string
	)

	return &redant.Command{
		Use:   "sync",
		Short: "rebase 到 base 并 force-with-lease 推送",
		Options: redant.OptionSet{
			{Flag: "dry-run", Description: "只输出将执行的 git 步骤", Value: redant.BoolOf(&dryRun)},
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
			{Flag: "base", Description: "rebase 目标（默认自动探测）", Value: redant.StringOf(&baseRef)},
			{Flag: "update-body", Description: "sync 后根据最新 diff 更新 PR 标题与正文", Value: redant.BoolOf(&updateBody)},
			{Flag: "ai", Description: "更新 PR 正文时使用 AI 润色", Value: redant.BoolOf(&useAI)},
			{Flag: "ai-provider", Description: "AI 提供方 auto|openai|copilot", Value: redant.StringOf(&aiProvider), Default: "auto"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}

			rc, err := LoadRepoContext(ctx, repoRoot)
			if err != nil {
				return err
			}
			if base := strings.TrimSpace(baseRef); base != "" {
				rc.BaseRef, err = detectBaseRef(ctx, repoRoot, base)
				if err != nil {
					return err
				}
			}

			_, _ = fmt.Fprintf(inv.Stdout, "sync plan:\n  1. git fetch origin %s\n  2. git rebase %s\n  3. git push --force-with-lease\n", rc.BaseRef, rc.BaseRef)
			if updateBody {
				_, _ = fmt.Fprintln(inv.Stdout, "  4. gh pr edit (regenerate title/body)")
			}
			if dryRun {
				if updateBody {
					draft, err := BuildDraft(ctx, rc)
					if err != nil {
						return err
					}
					if useAI {
						provider := aiprovider.ResolveProvider(aiProvider, repoRoot)
						enhanced, ok, aiErr := EnhanceDraft(ctx, provider, draft)
						if aiErr == nil && ok {
							draft = enhanced
						}
					}
					_, _ = fmt.Fprintf(inv.Stdout, "\nupdated title: %s\n\n%s\n", draft.Title, draft.Body)
				}
				_, _ = fmt.Fprintln(inv.Stdout, "dry-run complete (no git changes)")
				return nil
			}

			gh := NewGhClient(repoRoot)
			if err := gh.SyncPR(ctx, rc, false); err != nil {
				return err
			}
			if !updateBody {
				return nil
			}
			if err := gh.EnsureAvailable(ctx); err != nil {
				return err
			}

			draft, err := BuildDraft(ctx, rc)
			if err != nil {
				return err
			}
			if useAI {
				provider := aiprovider.ResolveProvider(aiProvider, repoRoot)
				enhanced, ok, aiErr := EnhanceDraft(ctx, provider, draft)
				if aiErr != nil {
					_, _ = fmt.Fprintf(inv.Stdout, "ai enhance skipped: %v\n", aiErr)
				} else if ok {
					draft = enhanced
				}
			}
			if err := gh.EditPR(ctx, draft); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(inv.Stdout, "pr body updated")
			return nil
		},
	}
}

func newMergeCommand() *redant.Command {
	var (
		dryRun bool
		repo   string
		method string
	)

	return &redant.Command{
		Use:   "merge",
		Short: "合并当前分支 PR",
		Options: redant.OptionSet{
			{Flag: "dry-run", Description: "只输出将执行的 merge 命令", Value: redant.BoolOf(&dryRun)},
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
			{Flag: "method", Description: "合并方式 squash|merge|rebase", Value: redant.StringOf(&method), Default: "squash"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}

			rc, err := LoadRepoContext(ctx, repoRoot)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(inv.Stdout, "branch: %s\n", rc.Branch)

			if dryRun {
				_, _ = fmt.Fprintf(inv.Stdout, "dry-run: would run `gh pr merge --%s`\n", strings.TrimSpace(method))
				return nil
			}

			gh := NewGhClient(repoRoot)
			if err := gh.EnsureAvailable(ctx); err != nil {
				return err
			}
			out, err := gh.MergePR(ctx, method, false)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(inv.Stdout, out)
			return nil
		},
	}
}

func resolveRepoRoot(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		repo = wd
	}
	return repo, nil
}
