package docscmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/redant"
)

func New() *redant.Command {
	root := &redant.Command{
		Use:   "docs",
		Short: "初始化和维护仓库文档模板",
		Long:  "初始化文档相关的 prompt / instruction 模板，便于通过 Copilot 维护 README 与 docs。",
	}

	root.Children = []*redant.Command{newInitCommand()}
	return root
}

func newInitCommand() *redant.Command {
	var (
		repoPath string
		force    bool
	)

	return &redant.Command{
		Use:   "init",
		Short: "初始化 documentation prompt / instruction 模板",
		Options: redant.OptionSet{
			{Flag: "repo", Description: "目标仓库目录（默认当前目录）", Value: redant.StringOf(&repoPath)},
			{Flag: "force", Description: "覆盖已有模板文件", Value: redant.BoolOf(&force), Default: "false"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			repoRoot, err := resolveRepoRoot(strings.TrimSpace(repoPath))
			if err != nil {
				return err
			}

			result, err := ensureDocumentationScaffold(repoRoot, scaffoldOptions{Force: force})
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
				_, _ = fmt.Fprintln(inv.Stdout, "documentation scaffold already up to date")
			}
			_, _ = fmt.Fprintf(inv.Stdout, "repo: %s\n", repoRoot)
			return nil
		},
	}
}
