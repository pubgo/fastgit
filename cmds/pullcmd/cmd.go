package pullcmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/redant"
)

type cmdParams struct {
	OpenaiClient *utils.OpenaiClient
}

func New() *redant.Command {
	var flagData = new(struct {
		pullAll bool
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
				return
			}

			if flagData.pullAll {
				utils.GitPull(ctx, "--all").Must()
			} else {
				utils.GitBranchSetUpstream(ctx, utils.GetBranchName()).Must()

				err := utils.GitPull(ctx, "origin", utils.GetBranchName()).GetErr()
				if err != nil {
					if isMergeConflict() {
						handleMergeConflict()
					} else {
						os.Exit(1)
					}
				}
			}
			return
		},
	}

	return app
}

func shouldPullDueToRemoteUpdate(msg string) bool {
	return strings.Contains(msg, "stale info") ||
		strings.Contains(msg, "[rejected]") ||
		strings.Contains(msg, "failed to push") ||
		strings.Contains(msg, "remote rejected")
}

// 执行 git pull（默认 merge 模式）
func gitPull() error {
	cmd := exec.Command("git", "pull", "--no-rebase")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// 检查是否存在未解决的合并冲突（U=unmerged）
func isMergeConflict() bool {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// 处理合并冲突：打开编辑器让用户解决
func handleMergeConflict() {
	fmt.Println("❌ Merge conflicts detected! Please resolve them.")

	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, _ := cmd.Output()
	files := strings.Split(strings.TrimSpace(string(output)), "\n")

	editor := getEditor()

	for _, file := range files {
		if file == "" {
			continue
		}
		fmt.Printf("📝 Conflict in file: %s\n", file)

		editCmd := exec.Command(editor, file)
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
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}
