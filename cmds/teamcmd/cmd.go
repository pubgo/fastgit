package teamcmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pubgo/fastgit/pkg/repoconfig"
	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/redant"
)

// New creates team governance commands.
func New() *redant.Command {
	root := &redant.Command{
		Use:   "team",
		Short: "团队仓库配置：policy / commit 模板",
	}

	root.Children = []*redant.Command{
		newInitCommand(),
		newValidateCommand(),
	}
	return root
}

func newInitCommand() *redant.Command {
	var repo string
	return &redant.Command{
		Use:   "init",
		Short: "初始化 .fastgit/policy.yaml 与 .fastgit/commit.yaml",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}
			created, err := repoconfig.InitScaffold(repoRoot)
			if err != nil {
				return err
			}
			if len(created) == 0 {
				_, _ = fmt.Fprintln(inv.Stdout, "team config already exists")
			}
			for _, path := range created {
				_, _ = fmt.Fprintf(inv.Stdout, "created: %s\n", path)
			}
			return nil
		},
	}
}

func newValidateCommand() *redant.Command {
	var (
		repo    string
		branch  string
		message string
	)
	return &redant.Command{
		Use:   "validate",
		Short: "按 .fastgit 规则校验分支名或 commit message",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "仓库目录（默认当前目录）", Value: redant.StringOf(&repo)},
			{Flag: "branch", Description: "待校验分支名（默认当前分支）", Value: redant.StringOf(&branch)},
			{Flag: "message", Description: "待校验 commit message", Value: redant.StringOf(&message)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			repoRoot, err := resolveRepoRoot(repo)
			if err != nil {
				return err
			}
			cfg, err := repoconfig.Load(repoRoot)
			if err != nil {
				return err
			}

			if strings.TrimSpace(branch) == "" && strings.TrimSpace(message) == "" {
				branch = utils.GetBranchName()
			}

			if strings.TrimSpace(branch) != "" {
				if err := cfg.ValidateBranch(branch); err != nil {
					return err
				}
				if cfg.IsProtectedBranch(branch) {
					_, _ = fmt.Fprintf(inv.Stdout, "warning: branch %q is protected\n", branch)
				}
				_, _ = fmt.Fprintf(inv.Stdout, "branch ok: %s\n", branch)
			}

			if strings.TrimSpace(message) != "" {
				if err := cfg.ValidateCommitMessage(message); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(inv.Stdout, "commit message ok\n")
			}
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
		return wd, nil
	}
	return repo, nil
}
