package checkcmd

import (
	"fmt"

	"github.com/pubgo/fastgit/pkg/repoconfig"
	"github.com/pubgo/redant"
)

func warnSensitiveStaged(inv *redant.Invocation, repoRoot string, stagedFiles []string) {
	if inv == nil || len(stagedFiles) == 0 {
		return
	}
	cfg, err := repoconfig.Load(repoRoot)
	if err != nil {
		return
	}
	for _, file := range stagedFiles {
		if cfg.MatchesSensitivePath(file) {
			_, _ = fmt.Fprintf(inv.Stdout, "policy warning: sensitive path staged: %s\n", file)
		}
	}
}
