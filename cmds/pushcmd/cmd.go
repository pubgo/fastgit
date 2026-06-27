package pushcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pubgo/fastgit/pkg/repoconfig"
	"github.com/pubgo/fastgit/pkg/workflow"
	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/redant"
)

func New() *redant.Command {
	var flagData = new(struct {
		pushAll         bool
		pushForce       bool
		overridePolicy  bool
	})

	return &redant.Command{
		Use:   "push",
		Short: "git push to remote origin",
		Options: []redant.Option{
			{
				Flag:        "all",
				Description: "push all branches",
				Value:       redant.BoolOf(&flagData.pushAll),
			},
			{
				Flag:        "force",
				Description: "force push current branch with --force-with-lease",
				Value:       redant.BoolOf(&flagData.pushForce),
			},
			{
				Flag:        "override-policy",
				Description: "bypass protected branch push block from .fastgit/policy.yaml",
				Value:       redant.BoolOf(&flagData.overridePolicy),
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) (gErr error) {
			defer result.RecoveryErr(&gErr, func(err error) error {
				if errors.Is(err, context.Canceled) {
					return nil
				}

				if err.Error() == "signal: interrupt" {
					return nil
				}

				return err
			})

			command := i.Command
			if len(command.Args) > 0 {
				log.Error(ctx).Msgf("unknown command:%v", command.Args)
				return redant.DefaultHelpFn()(ctx, i)
			}

			utils.LogConfigAndBranch()

			repoRoot, err := gitRepoRoot()
			if err != nil {
				return err
			}
			branch := utils.GetBranchName()
			bundle, err := repoconfig.Load(repoRoot)
			if err != nil {
				return err
			}
			if err := bundle.ValidatePush(branch, flagData.overridePolicy); err != nil {
				return err
			}

			if flagData.pushAll && flagData.pushForce {
				return errors.Errorf("--force cannot be used with --all")
			}

			var pushErr error
			if flagData.pushAll {
				pushErr = utils.ShellExec(ctx, "git", "push", "--all", "origin")
			} else if flagData.pushForce {
				pushErr = utils.ShellExec(ctx, "git", "push", "--force-with-lease", "--set-upstream", "origin", branch)
			} else {
				pushErr = utils.ShellExec(ctx, "git", "push", "--set-upstream", "origin", branch)
			}
			if pushErr == nil {
				workflow.PrintRecommendations(os.Stdout, "push")
			}
			return pushErr
		},
	}
}

func gitRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
