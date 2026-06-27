package fastcommitcmd

import (
	"fmt"

	"github.com/pubgo/fastgit/pkg/repoconfig"
	"github.com/pubgo/fastgit/utils"
)

func enforceRepoPolicy(repoCfg repoconfig.Bundle, branch, message string, skipPolicy bool) error {
	if err := repoCfg.CheckBranch(branch, skipPolicy); err != nil {
		return err
	}
	if err := repoCfg.CheckCommitMessage(message, skipPolicy); err != nil {
		return err
	}
	return nil
}

func warnRepoPolicy(repoCfg repoconfig.Bundle, branch, message string) {
	if err := repoCfg.ValidateBranch(branch); err != nil {
		fmt.Printf("policy warning (branch): %v\n", err)
	}
	if err := repoCfg.WarnCommitMessage(message); err != nil {
		fmt.Printf("policy warning (commit): %v\n", err)
	}
}

func currentBranch() string {
	return utils.GetBranchName()
}
