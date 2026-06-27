package fastcommitcmd

import (
	"context"
	"fmt"

	"github.com/pubgo/fastgit/cmds/checkcmd"
	"github.com/pubgo/fastgit/pkg/repoconfig"
)

func runPreCommitCheck(ctx context.Context, repoRoot string, skip bool) error {
	if skip {
		return nil
	}
	cfg := checkcmd.LoadConfig(repoRoot)
	_, err := checkcmd.Run(ctx, cfg, checkcmd.RunOptions{
		StagedOnly: true,
		RepoRoot:   repoRoot,
	})
	if err != nil {
		return fmt.Errorf("pre-commit check failed: %w\nhint: fix issues, or use --skip-check to bypass", err)
	}
	return nil
}

func ensurePushPolicy(repoRoot, branch string, override bool) error {
	cfg, err := repoconfig.Load(repoRoot)
	if err != nil {
		return err
	}
	return cfg.ValidatePush(branch, override)
}
