package copilotcmd

import (
	"os"

	copilot "github.com/github/copilot-sdk/go"
	"github.com/pubgo/fastgit/pkg/copilotperm"
)

func buildPermissionHandler(mode string) copilot.PermissionHandlerFunc {
	parsed, err := copilotperm.ParseMode(mode)
	if err != nil {
		parsed = copilotperm.DefaultMode
	}

	auditor, _ := copilotperm.NewFileAuditor()
	prompter := copilotperm.PrompterForMode(parsed, copilotperm.DefaultTerminalPrompter(os.Stdin, os.Stdout))
	return copilotperm.NewHandler(copilotperm.Options{
		Mode:     parsed,
		Auditor:  auditor,
		Prompter: prompter,
	})
}
