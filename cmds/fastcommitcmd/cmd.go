package fastcommitcmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pubgo/dix/v2"
	"github.com/pubgo/dix/v2/dixcontext"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/redant"
	"github.com/sashabaranov/go-openai"
	"github.com/yarlson/tap"

	"github.com/pubgo/fastcommit/utils"
)

type Config struct {
	GenVersion bool `yaml:"gen_version"`
}

type cmdParams struct {
	OpenaiClient *utils.OpenaiClient
	CommitCfg    []*Config
}

func New() *redant.Command {
	var flags = new(struct {
		showPrompt bool
		fastCommit bool
		amend      bool
	})

	app := &redant.Command{
		Use:   "commit",
		Short: "Intelligent generation of git commit message",
		Options: []redant.Option{
			{
				Flag:        "prompt",
				Description: "Show prompt.",
				Value:       redant.BoolOf(&flags.showPrompt),
			},
			{
				Flag:        "fast",
				Description: "Quickly generate messages without prompts.",
				Value:       redant.BoolOf(&flags.fastCommit),
			},
			{
				Flag:        "amend",
				Description: "Amend the last commit.",
				Value:       redant.BoolOf(&flags.amend),
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) (gErr error) {
			di := dixcontext.Get(ctx)
			var params cmdParams
			params = dix.Inject(di, params)

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

			res := utils.PreGitPush(ctx)
			if res != "" {
				if shouldPullDueToRemoteUpdate(res) {
					err := gitPull()
					if err != nil {
						if isMergeConflict() {
							handleMergeConflict()
						} else {
							os.Exit(1)
						}
					} else {
						informUserToAmendAndPush()
					}
				}
			}

			isDirty := utils.IsDirty().Unwrap()
			if !isDirty {
				return
			}

			//username := strings.TrimSpace(assert.Must1(utils.ShellExecOutput("git", "config", "get", "user.name")))

			if flags.fastCommit {
				preMsg := strings.TrimSpace(utils.ShellExecOutput(ctx, "git", "log", "-1", "--pretty=%B").Unwrap())
				prefixMsg := fmt.Sprintf("chore: quick update %s", utils.GetBranchName())
				msg := fmt.Sprintf("%s at %s", prefixMsg, time.Now().Format(time.DateTime))

				msg = strings.TrimSpace(tap.Text(ctx, tap.TextOptions{
					Message:      "git message(update or enter):",
					InitialValue: msg,
					DefaultValue: msg,
					Placeholder:  "update or enter",
				}))

				if msg == "" {
					return
				}

				assert.Must(utils.ShellExec(ctx, "git", "add", "-A"))
				res := utils.ShellExecOutput(ctx, "git", "status").Unwrap()

				if !flags.amend {
					assert.Must(utils.ShellExec(ctx, "git", "commit", "-m", strconv.Quote(msg)))
				} else {
					if strings.Contains(preMsg, prefixMsg) && !strings.Contains(res, `(use "git commit" to conclude merge)`) {
						assert.Must(utils.ShellExec(ctx, "git", "commit", "--amend", "--no-edit", "-m", strconv.Quote(msg)))
					} else {
						assert.Must(utils.ShellExec(ctx, "git", "commit", "-m", strconv.Quote(msg)))
					}
				}

				res = utils.GitPush(ctx, "--force-with-lease", "origin", utils.GetBranchName())
				if shouldPullDueToRemoteUpdate(res) {
					err := gitPull()
					if err != nil {
						if isMergeConflict() {
							handleMergeConflict()
						} else {
							os.Exit(1)
						}
					} else {
						informUserToAmendAndPush()
					}
				}
				return
			}

			// 非快速提交模式：遍历git log，将非prefixMsg开头的提交合并为一次提交
			prefixMsg := fmt.Sprintf("chore: quick update %s", utils.GetBranchName())
			commitsToSquash := getCommitsToSquash(ctx, prefixMsg)

			// 如果有需要合并的提交，先重置到第一个提交之前
			if len(commitsToSquash) > 0 {
				// 获取第一个提交的父提交
				parentCommit := getParentCommit(ctx, commitsToSquash[0])
				if parentCommit != "" {
					// 重置到第一个提交的父提交
					utils.ShellExec(ctx, "git", "reset", "--soft", parentCommit)
				} else {
					// 如果没有父提交（即第一个提交），重置到初始状态
					utils.ShellExec(ctx, "git", "reset", "--soft", "HEAD~"+strconv.Itoa(len(commitsToSquash)))
				}
			} else {
				// 没有需要合并的提交，添加所有变更
				assert.Must(utils.ShellExec(ctx, "git", "add", "--update"))
			}

			// 获取当前所有变动的文件（重置后的工作区状态）
			diffResult := utils.GetStagedDiff(ctx).Unwrap()
			if diffResult == nil || len(diffResult.Files) == 0 {
				return nil
			}

			log.Info().Msg(utils.GetDetectedMessage(diffResult.Files))
			for _, file := range diffResult.Files {
				log.Info().Msg("file: " + file)
			}

			// 使用spinner生成提交信息
			s := spinner.New(spinner.CharSets[35], 100*time.Millisecond, func(s *spinner.Spinner) {
				s.Prefix = "generate git message: "
			})
			s.Start()
			generatePrompt := utils.GeneratePrompt("en", 50, utils.ConventionalCommitType)
			resp, err := params.OpenaiClient.Client.CreateChatCompletion(
				ctx,
				openai.ChatCompletionRequest{
					Model: params.OpenaiClient.Cfg.Model,
					Messages: []openai.ChatCompletionMessage{
						{
							Role:    openai.ChatMessageRoleSystem,
							Content: generatePrompt,
						},
						{
							Role:    openai.ChatMessageRoleUser,
							Content: diffResult.Diff,
						},
					},
				},
			)
			s.Stop()

			if err != nil {
				log.Err(err).Msg("failed to call openai")
				return errors.WrapCaller(err)
			}

			if len(resp.Choices) == 0 {
				return nil
			}

			msg := resp.Choices[0].Message.Content
			msg = strings.TrimSpace(tap.Text(ctx, tap.TextOptions{
				Message:      "git message(update or enter):",
				InitialValue: msg,
				DefaultValue: msg,
				Placeholder:  "update or enter",
			}))

			if msg == "" {
				return
			}

			// 创建新的提交
			assert.Must(utils.ShellExec(ctx, "git", "commit", "-m", strconv.Quote(msg)))
			utils.GitPush(ctx, "origin", utils.GetBranchName())
			//if flags.showPrompt {
			fmt.Println("\n" + generatePrompt + "\n")
			//}
			log.Info().Any("usage", resp.Usage).Msg("openai response usage")
			return
		},
	}

	return app
}

// getCommitsToSquash 遍历git log，找到以prefixMsg开头的提交（这些是需要合并的提交）
func getCommitsToSquash(ctx context.Context, prefixMsg string) []string {
	// 获取当前分支最近的提交列表，直到遇到不是prefixMsg开头的提交
	branchName := utils.GetBranchName()
	cmd := exec.CommandContext(ctx, "git", "log", branchName, "--oneline", "--pretty=format:%H%s", "-10") // 限制最近10个提交
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commitsToSquash []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		commitHash := parts[0]
		commitMsg := parts[1]

		// 如果提交消息以prefixMsg开头，添加到待合并列表
		if strings.HasPrefix(commitMsg, prefixMsg) {
			commitsToSquash = append(commitsToSquash, commitHash)
		} else {
			// 如果遇到不是prefixMsg开头的提交，停止遍历
			break
		}
	}

	return commitsToSquash
}

// getParentCommit 获取指定提交的父提交
func getParentCommit(ctx context.Context, commitHash string) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", commitHash+"^")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
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
