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

			isDirty := utils.IsDirty().Unwrap()
			if isDirty {
				return nil
			}

			if flagData.pushAll && flagData.pushForce {
				return errors.Errorf("--force cannot be used with --all")
			}

			if flagData.pushAll {
				utils.GitPush(ctx, "--all", "origin")
				return nil
			}

			if flagData.pushForce {
				utils.GitPush(ctx, "--force-with-lease", "--set-upstream", "origin", utils.GetBranchName())
				return nil
			}

			utils.GitPush(ctx, "--set-upstream", "origin", utils.GetBranchName())
			return nil
		},
	}
}
