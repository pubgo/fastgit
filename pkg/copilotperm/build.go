package copilotperm

import (
	"io"
	"os"

	copilot "github.com/github/copilot-sdk/go"
)

// BuildHandler resolves mode and returns a Copilot OnPermissionRequest handler.
func BuildHandler(cliFlag string, commandDefault Mode, in io.Reader, out io.Writer) copilot.PermissionHandlerFunc {
	mode, err := ResolveMode(cliFlag, commandDefault)
	if err != nil {
		mode = commandDefault
		if mode == "" {
			mode = DefaultMode
		}
	}

	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}

	auditor, _ := NewFileAuditor()
	prompter := PrompterForMode(mode, DefaultTerminalPrompter(in, out))
	return NewHandler(Options{
		Mode:     mode,
		Auditor:  auditor,
		Prompter: prompter,
	})
}
