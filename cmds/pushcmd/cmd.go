package pushcmd

import (
	"context"

	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/redant"
)

func New() *redant.Command {
	var flagData = new(struct {
		pushAll   bool
		pushForce bool
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

			if flagData.pushAll && flagData.pushForce {
				return errors.Errorf("--force cannot be used with --all")
			}

			if flagData.pushAll {
				return utils.ShellExec(ctx, "git", "push", "--all", "origin")
			}

			if flagData.pushForce {
				return utils.ShellExec(ctx, "git", "push", "--force-with-lease", "--set-upstream", "origin", utils.GetBranchName())
			}

			return utils.ShellExec(ctx, "git", "push", "--set-upstream", "origin", utils.GetBranchName())
		},
	}
}
