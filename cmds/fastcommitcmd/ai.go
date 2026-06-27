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
	"github.com/yarlson/tap"

	"github.com/pubgo/fastgit/pkg/aiprovider"
	"github.com/pubgo/fastgit/pkg/gitconflict"
	"github.com/pubgo/fastgit/pkg/repoconfig"
	"github.com/pubgo/fastgit/pkg/workflow"
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
				if gitconflict.HasConflicts(ctx, "") {
					handleMergeConflict(ctx)
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

		repoRoot := mustRepoRoot()
		repoCfg, _ := repoconfig.Load(repoRoot)
		if err := enforceRepoPolicy(repoCfg, currentBranch(), msg, flags.skipPolicy); err != nil {
			return err
		}
		warnRepoPolicy(repoCfg, currentBranch(), msg)

		assert.Must(utils.ShellExec(ctx, "git", "add", "-A"))
		res := utils.ShellExecOutput(ctx, "git", "status").Unwrap()

		if err := runPreCommitCheck(ctx, mustRepoRoot(), flags.skipCheck); err != nil {
			return err
		}

		if !flags.amend {
			assert.Must(utils.ShellExec(ctx, "git", "commit", "-m", strconv.Quote(msg)))
		} else {
			if strings.Contains(preMsg, prefixMsg) && !strings.Contains(res, `(use "git commit" to conclude merge)`) {
				assert.Must(utils.ShellExec(ctx, "git", "commit", "--amend", "--no-edit", "-m", strconv.Quote(msg)))
			} else {
				assert.Must(utils.ShellExec(ctx, "git", "commit", "-m", strconv.Quote(msg)))
			}
		}

		if err := ensurePushPolicy(mustRepoRoot(), utils.GetBranchName(), flags.overridePolicy); err != nil {
			return err
		}
		res = utils.GitPush(ctx, "--force-with-lease", "origin", utils.GetBranchName())
		if shouldPullDueToRemoteUpdate(res) {
			err := gitPull()
			if err != nil {
				if gitconflict.HasConflicts(ctx, "") {
					handleMergeConflict(ctx)
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

	repoRoot := mustRepoRoot()
	if err := runPreCommitCheck(ctx, repoRoot, flags.skipCheck); err != nil {
		return err
	}

	repoCfg, _ := repoconfig.Load(repoRoot)
	if err := repoCfg.CheckBranch(currentBranch(), flags.skipPolicy); err != nil {
		return err
	}
	for _, file := range diffResult.Files {
		if repoCfg.MatchesSensitivePath(file) {
			log.Warn().Str("file", file).Msg("sensitive path staged — review carefully")
		}
	}

	log.Info().Msg(utils.GetDetectedMessage(diffResult.Files))
	for _, file := range diffResult.Files {
		log.Info().Msg("file: " + file)
	}

	s := spinner.New(spinner.CharSets[35], 100*time.Millisecond, func(s *spinner.Spinner) {
		s.Prefix = "generate git message: "
	})
	s.Start()
	locale := "en"
	maxLength := 50
	if repoCfg.Commit.Locale != "" {
		locale = repoCfg.Commit.Locale
	}
	if repoCfg.Commit.MaxLength > 0 {
		maxLength = repoCfg.Commit.MaxLength
	}
	generatePrompt := utils.AppendAllowedTypes(
		utils.GeneratePrompt(locale, maxLength, utils.ConventionalCommitType),
		repoCfg.Commit.Types,
	)

	useCandidates := shouldUseCandidates(flags, repoCfg, params)
	var msg string
	if useCandidates {
		candidates, err := aiprovider.GenerateCommitCandidates(ctx, params.AI, diffResult.Diff)
		s.Stop()
		if err != nil {
			log.Err(err).Msg("failed to generate commit candidates")
		}
		if hint := aiprovider.BreakingChangeHint(diffResult.Diff); hint != "" {
			log.Warn().Msg(hint)
			fmt.Println(hint)
		}
		options := make([]tap.SelectOption[string], 0, len(candidates))
		for _, candidate := range candidates {
			candidate := candidate
			options = append(options, tap.SelectOption[string]{
				Label: aiprovider.FormatCandidateLabel(candidate),
				Value: candidate.Message,
			})
		}
		if len(options) == 0 {
			return nil
		}
		selected := tap.Select[string](ctx, tap.SelectOptions[string]{
			Message: "Pick a commit message:",
			Options: options,
		})
		msg = strings.TrimSpace(selected)
	} else {
		aiResp, err := params.AI.Complete(ctx, aiprovider.CompleteRequest{
			System: generatePrompt,
			User:   diffResult.Diff,
		})
		s.Stop()

		if err != nil {
			log.Err(err).Msg("failed to generate commit message")
			return errors.WrapCaller(err)
		}

		if aiResp.Fallback {
			log.Warn().Str("provider", aiResp.Provider).Msg("using rule-based commit message fallback (AI unavailable)")
		}
		if hint := aiprovider.BreakingChangeHint(diffResult.Diff); hint != "" {
			log.Warn().Msg(hint)
			fmt.Println(hint)
		}

		msg = strings.TrimSpace(tap.Text(ctx, tap.TextOptions{
			Message:      "git message(update or enter):",
			InitialValue: aiResp.Text,
			DefaultValue: aiResp.Text,
			Placeholder:  "update or enter",
		}))
	}
	if msg == "" {
		return nil
	}

	if err := enforceRepoPolicy(repoCfg, currentBranch(), msg, flags.skipPolicy); err != nil {
		return err
	}
	warnRepoPolicy(repoCfg, currentBranch(), msg)

	assert.Must(utils.ShellExec(ctx, "git", "commit", "-m", strconv.Quote(msg)))
	if err := ensurePushPolicy(repoRoot, utils.GetBranchName(), flags.overridePolicy); err != nil {
		return err
	}
	utils.GitPush(ctx, "--force-with-lease", "origin", utils.GetBranchName())
	if flags.showPrompt && !useCandidates {
		fmt.Println("\n" + generatePrompt + "\n")
	}
	log.Info().Str("message", msg).Bool("candidates", useCandidates).Msg("commit message generated")
	workflow.PrintRecommendations(os.Stdout, "commit")
	return nil
}

func mustRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func shouldUseCandidates(flags *flagOptions, repoCfg repoconfig.Bundle, params cmdParams) bool {
	if flags != nil && flags.single {
		return false
	}
	if flags != nil && flags.candidates {
		return true
	}
	if repoCfg.Commit.CandidatesDefault {
		return true
	}
	for _, cfg := range params.CommitCfg {
		if cfg != nil && cfg.CandidatesDefault {
			return true
		}
	}
	return false
}
