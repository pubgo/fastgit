package worktreecmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/redant"
)

func New() *redant.Command {
	var createFlags = struct {
		base string
	}{
		base: "main",
	}

	var removeFlags = struct {
		path bool
	}{}

	listHandler := func(ctx context.Context, i *redant.Invocation) error {
		worktrees, err := utils.ListWorktrees()
		if err != nil {
			return err
		}

		fmt.Printf("%-1s %-20s %-10s %s\n", " ", "BRANCH", "COMMIT", "PATH")
		for _, wt := range worktrees {
			marker := " "
			if wt.IsCurrent {
				marker = "*"
			}

			branch := wt.Branch
			if wt.IsDetached || branch == "" {
				branch = "(detached)"
			}

			commit := wt.Commit
			if len(commit) > 8 {
				commit = commit[:8]
			}

			fmt.Printf("%-1s %-20s %-10s %s\n", marker, branch, commit, wt.Path)
		}
		return nil
	}

	return &redant.Command{
		Use:   "worktree",
		Short: "Manage git worktrees",
		Children: []*redant.Command{
			{
				Use:   "list",
				Short: "List all worktrees",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					if len(i.Command.Args) > 0 {
						return redant.DefaultHelpFn()(ctx, i)
					}
					return listHandler(ctx, i)
				},
			},
			{
				Use:   "create",
				Short: "Create a worktree from an issue id/branch",
				Options: []redant.Option{
					{
						Flag:        "base",
						Description: "Base branch to create from",
						Value:       redant.StringOf(&createFlags.base),
						Default:     "main",
					},
				},
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					args := commandArgs(i)
					if len(args) != 1 {
						return redant.DefaultHelpFn()(ctx, i)
					}

					path, err := utils.CreateWorktree(args[0], createFlags.base)
					if err != nil {
						return err
					}

					fmt.Printf("created worktree: %s\n", path)
					return nil
				},
			},
			{
				Use:   "remove",
				Short: "Remove a worktree by issue id/branch or by path",
				Options: []redant.Option{
					{
						Flag:        "path",
						Description: "Treat argument as an absolute/relative worktree path",
						Value:       redant.BoolOf(&removeFlags.path),
					},
				},
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					args := commandArgs(i)
					if len(args) != 1 {
						return redant.DefaultHelpFn()(ctx, i)
					}

					var err error
					if removeFlags.path {
						err = utils.RemoveWorktreeByPath(args[0])
					} else {
						err = utils.RemoveWorktree(args[0])
					}

					if err != nil {
						return err
					}

					fmt.Printf("removed worktree: %s\n", args[0])
					return nil
				},
			},
		},
		Handler: listHandler,
	}
}

func commandArgs(i *redant.Invocation) []string {
	args := make([]string, 0, len(i.Command.Args))
	for _, arg := range i.Command.Args {
		value := strings.TrimSpace(arg.Value.String())
		if value != "" {
			args = append(args, value)
		}
	}
	return args
}
