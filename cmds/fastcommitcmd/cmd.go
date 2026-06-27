package fastcommitcmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/redant"

	"github.com/pubgo/fastgit/pkg/aiprovider"
	"github.com/pubgo/fastgit/pkg/gitconflict"
	"github.com/pubgo/fastgit/utils"
)

type flagOptions struct {
	showPrompt     bool
	fastCommit     bool
	amend          bool
	candidates     bool
	single         bool
	skipCheck      bool
	skipPolicy     bool
	overridePolicy bool
}

type Config struct {
	GenVersion        bool `yaml:"gen_version"`
	CandidatesDefault bool `yaml:"candidates_default"`
}

type cmdParams struct {
	AI        aiprovider.Provider
	CommitCfg []*Config
}

func New() *redant.Command {
	var flags = new(flagOptions)

	app := &redant.Command{
		Use:   "commit",
		Short: "Intelligent generation of git commit message",
		Children: []*redant.Command{
			{
				Use:   "ai",
				Short: "AI powered commit flow",
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
					{
						Flag:        "candidates",
						Description: "Generate 3 commit message candidates to pick from.",
						Value:       redant.BoolOf(&flags.candidates),
					},
					{
						Flag:        "single",
						Description: "Generate a single commit message (skip multi-candidate picker).",
						Value:       redant.BoolOf(&flags.single),
					},
					{
						Flag:        "skip-check",
						Description: "Skip pre-commit quality check (fastgit check run --staged-only).",
						Value:       redant.BoolOf(&flags.skipCheck),
					},
					{
						Flag:        "skip-policy",
						Description: "Bypass .fastgit/policy.yaml hard enforcement.",
						Value:       redant.BoolOf(&flags.skipPolicy),
					},
					{
						Flag:        "override-policy",
						Description: "Bypass protected branch push block from .fastgit/policy.yaml.",
						Value:       redant.BoolOf(&flags.overridePolicy),
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

					if len(i.Command.Args) > 0 {
						log.Error(ctx).Msgf("unknown command:%v", i.Command.Args)
						return redant.DefaultHelpFn()(ctx, i)
					}

					return runAICommit(ctx, flags)
				},
			},
		},
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
			{
				Flag:        "candidates",
				Description: "Generate 3 commit message candidates to pick from.",
				Value:       redant.BoolOf(&flags.candidates),
			},
			{
				Flag:        "single",
				Description: "Generate a single commit message (skip multi-candidate picker).",
				Value:       redant.BoolOf(&flags.single),
			},
			{
				Flag:        "skip-check",
				Description: "Skip pre-commit quality check (fastgit check run --staged-only).",
				Value:       redant.BoolOf(&flags.skipCheck),
			},
			{
				Flag:        "skip-policy",
				Description: "Bypass .fastgit/policy.yaml hard enforcement.",
				Value:       redant.BoolOf(&flags.skipPolicy),
			},
			{
				Flag:        "override-policy",
				Description: "Bypass protected branch push block from .fastgit/policy.yaml.",
				Value:       redant.BoolOf(&flags.overridePolicy),
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

			return runAICommit(ctx, flags)
		},
	}

	return app
}

// getFirstNonPrefixCommit 获取第一个没有prefixMsg的提交ID
func getFirstNonPrefixCommit(ctx context.Context, prefixMsg string) string {
	// 获取当前分支最近的提交列表，找到第一个不是prefixMsg开头的提交
	branchName := utils.GetBranchName()
	cmd := exec.CommandContext(ctx, "git", "log", branchName, "--oneline", "--pretty=format:%H %s", "-20") // 增加到20个提交以确保找到
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
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

		// 如果提交消息不以prefixMsg开头，返回这个提交的hash
		if !strings.HasPrefix(commitMsg, prefixMsg) {
			return commitHash
		}
	}

	// 如果所有提交都以prefixMsg开头，返回空字符串
	return ""
}

// getCommitsToSquash 遍历git log，找到以prefixMsg开头的提交（这些是需要合并的提交）
func getCommitsToSquash(ctx context.Context, prefixMsg string) []string {
	// 获取当前分支最近的提交列表，直到遇到不是prefixMsg开头的提交
	branchName := utils.GetBranchName()
	cmd := exec.CommandContext(ctx, "git", "log", branchName, "--oneline", "--pretty=format:%H %s", "-10") // 限制最近10个提交
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
	_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
}
