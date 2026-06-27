package pullcmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/pubgo/fastgit/pkg/gitconflict"
	"github.com/pubgo/fastgit/pkg/workflow"
	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/redant"
	"mvdan.cc/sh/v3/shell"
)

func New() *redant.Command {
	var flagData = new(struct {
		pullAll bool
		hard    bool
	})
	app := &redant.Command{
		Use:   "pull",
		Short: "git pull from remote origin",
		Options: []redant.Option{
			{
				Flag:        "all",
				Description: "pull all branches",
				Value:       redant.BoolOf(&flagData.pullAll),
			},
			{
				Flag:        "hard",
				Description: "force sync current branch with remote via fetch + reset --hard",
				Value:       redant.BoolOf(&flagData.hard),
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

			if flagData.pullAll && flagData.hard {
				return errors.New("--hard cannot be used with --all")
			}

			if flagData.pullAll {
				return utils.GitPull(ctx, "--all").GetErr()
			}

			if flagData.hard {
				return hardSyncCurrentBranch(ctx, utils.GetBranchName())
			}

			isDirty := utils.IsDirty().Unwrap()
			if isDirty {
				return errors.New("working tree has uncommitted changes, use --hard to force sync or commit/stash first")
			}

			err := pullCurrentBranch(ctx, utils.GetBranchName())
			if err != nil {
				if gitconflict.HasConflicts(ctx, "") {
					handleMergeConflict(ctx)
					workflow.PrintRecommendations(os.Stdout, "pull")
					return nil
				}
				return err
			}
			workflow.PrintRecommendations(os.Stdout, "pull")
			return
		},
	}

	return app
}

func pullCurrentBranch(ctx context.Context, branch string) error {
	if hasUpstream() {
		return utils.GitPull(ctx).GetErr()
	}

	if err := utils.GitBranchSetUpstream(ctx, branch).GetErr(); err != nil {
		return utils.GitPull(ctx, "origin", branch).GetErr()
	}

	return utils.GitPull(ctx).GetErr()
}

func hasUpstream() bool {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	return cmd.Run() == nil
}

func getUpstreamRef() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func hardSyncCurrentBranch(ctx context.Context, branch string) error {
	upstream := fmt.Sprintf("origin/%s", branch)
	if up, err := getUpstreamRef(); err == nil && up != "" {
		upstream = up
	}

	remote, remoteBranch := splitRemoteRef(upstream)
	if err := utils.ShellExec(ctx, "git", "fetch", "--prune", remote, remoteBranch); err != nil {
		return err
	}
	return utils.ShellExec(ctx, "git", "reset", "--hard", upstream)
}

func splitRemoteRef(ref string) (remote, branch string) {
	parts := strings.Split(ref, "/")
	if len(parts) < 2 {
		return "origin", ref
	}

	remote = parts[0]
	branch = path.Clean(strings.Join(parts[1:], "/"))
	return remote, branch
}

// 处理合并冲突：输出摘要并打开编辑器
func handleMergeConflict(ctx context.Context) {
	snap, err := gitconflict.BuildSnapshot(ctx, "")
	if err != nil {
		fmt.Printf("conflict summary error: %v\n", err)
	} else {
		fmt.Println(snap.Summary)
	}

	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, _ := cmd.Output()
	files := strings.Split(strings.TrimSpace(string(output)), "\n")

	editor := getEditor()

	for _, file := range files {
		if file == "" {
			continue
		}
		fmt.Printf("📝 Conflict in file: %s\n", file)

		editorArgs := buildEditorCommand(editor, file)
		editCmd := exec.Command(editorArgs[0], editorArgs[1:]...)
		editCmd.Stdin = os.Stdin
		editCmd.Stdout = os.Stdout
		editCmd.Stderr = os.Stderr

		fmt.Printf("Opening editor '%s'...\n", editor)
		if err := editCmd.Run(); err != nil {
			log.Printf("Failed to edit %s: %v", file, err)
		}
	}

	// 提示用户完成后续操作
	informUserToAmendAndPush()
}

func getEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}

	if _, err := exec.LookPath("zed"); err == nil {
		return "zed -w"
	}

	if _, err := exec.LookPath("code"); err == nil {
		return "code -w"
	}

	if _, err := exec.LookPath("vim"); err == nil {
		return "vim"
	}

	if _, err := exec.LookPath("nano"); err == nil {
		return "nano"
	}
	return "vi"
}

func buildEditorCommand(editor, file string) []string {
	fields, err := shell.Fields(editor, nil)
	if err != nil || len(fields) == 0 {
		return []string{editor, file}
	}
	return append(fields, file)
}

// 提示用户如何继续
func informUserToAmendAndPush() {
	fmt.Println("\n----------------------------------------")
	fmt.Println("🛠️  Conflict resolved or pulled successfully.")
	fmt.Println("Now you can:")
	fmt.Println("   1. Review changes")
	fmt.Println("   2. Run 'git add .' to stage resolved files")
	fmt.Println("   3. Run 'git commit' (do NOT use --amend yet unless you want to absorb merge)")
	fmt.Println("   4. Then do:")
	fmt.Println("      git push --force-with-lease")
	fmt.Println("")
	fmt.Println("💡 Tip: 如果你想保持单个 commit，可以在 merge 后做交互式 rebase：")
	fmt.Println("    git reset HEAD~1   # 取消 merge commit")
	fmt.Println("    git add .")
	fmt.Println("    git commit --amend")
	fmt.Println("    git push --force-with-lease")
	fmt.Println("----------------------------------------")

	fmt.Println("\nPress Enter after you're done...")
	_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
}
