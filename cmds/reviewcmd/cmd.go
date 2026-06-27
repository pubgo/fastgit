package reviewcmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pubgo/fastgit/pkg/aiprovider"
	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/redant"
)

// New creates the review command group.
func New() *redant.Command {
	root := &redant.Command{
		Use:   "review",
		Short: "本地代码评审（staged diff）",
	}

	root.Children = []*redant.Command{newStagedCommand()}
	return root
}

func newStagedCommand() *redant.Command {
	var (
		dryRun     bool
		aiProvider string
	)

	return &redant.Command{
		Use:   "staged",
		Short: "对 staged diff 输出 Blockers/Suggestions/Nits",
		Options: redant.OptionSet{
			{Flag: "dry-run", Description: "只说明将评审的内容，不调用 AI", Value: redant.BoolOf(&dryRun)},
			{Flag: "ai-provider", Description: "AI 提供方 auto|openai|copilot", Value: redant.StringOf(&aiProvider), Default: "auto"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			diffResult := utils.GetStagedDiff(ctx).Unwrap()
			if diffResult == nil || strings.TrimSpace(diffResult.Diff) == "" {
				return fmt.Errorf("no staged changes; stage files before review")
			}

			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}

			provider := aiprovider.ResolveProvider(aiProvider, repoRoot)
			report, err := ReviewStaged(ctx, provider, diffResult.Diff, dryRun)
			if err != nil && !dryRun {
				_, _ = fmt.Fprintf(inv.Stdout, "ai review fallback: %v\n\n", err)
			}
			_, _ = fmt.Fprintln(inv.Stdout, report)
			return nil
		},
	}
}
