package fastcommitcmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pubgo/dix/v2"
	"github.com/pubgo/dix/v2/dixcontext"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/sashabaranov/go-openai"
	"github.com/yarlson/tap"

	"github.com/pubgo/fastgit/utils"
)

func runAICommit(ctx context.Context, flags *flagOptions) error {
	di := dixcontext.Get(ctx)
	var params cmdParams
	params = dix.Inject(di, params)

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

	if flags.fastCommit {
		isDirty := utils.IsDirty().Unwrap()
		if !isDirty {
			return nil
		}

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
			return nil
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
		return nil
	}

	prefixMsg := fmt.Sprintf("chore: quick update %s", utils.GetBranchName())
	targetCommit := getFirstNonPrefixCommit(ctx, prefixMsg)

	if targetCommit != "" {
		assert.Must(utils.ShellExec(ctx, "git", "reset", "--soft", targetCommit))
	} else {
		commitsToSquash := getCommitsToSquash(ctx, prefixMsg)
		if len(commitsToSquash) > 0 {
			parentCommit := getParentCommit(ctx, commitsToSquash[0])
			if parentCommit != "" {
				assert.Must(utils.ShellExec(ctx, "git", "reset", "--soft", parentCommit))
			} else {
				assert.Must(utils.ShellExec(ctx, "git", "reset", "--soft", "HEAD~"+strconv.Itoa(len(commitsToSquash))))
			}
		}
	}

	if utils.IsDirty().Unwrap() {
		assert.Must(utils.ShellExec(ctx, "git", "add", "--update"))
	}

	diffResult := utils.GetStagedDiff(ctx).Unwrap()
	if diffResult == nil || len(diffResult.Files) == 0 {
		return nil
	}

	log.Info().Msg(utils.GetDetectedMessage(diffResult.Files))
	for _, file := range diffResult.Files {
		log.Info().Msg("file: " + file)
	}

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
				{Role: openai.ChatMessageRoleSystem, Content: generatePrompt},
				{Role: openai.ChatMessageRoleUser, Content: diffResult.Diff},
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

	msg := strings.TrimSpace(tap.Text(ctx, tap.TextOptions{
		Message:      "git message(update or enter):",
		InitialValue: resp.Choices[0].Message.Content,
		DefaultValue: resp.Choices[0].Message.Content,
		Placeholder:  "update or enter",
	}))
	if msg == "" {
		return nil
	}

	assert.Must(utils.ShellExec(ctx, "git", "commit", "-m", strconv.Quote(msg)))
	utils.GitPush(ctx, "--force-with-lease", "origin", utils.GetBranchName())
	if flags.showPrompt {
		fmt.Println("\n" + generatePrompt + "\n")
	}
	log.Info().Any("usage", resp.Usage).Msg("openai response usage")
	return nil
}
