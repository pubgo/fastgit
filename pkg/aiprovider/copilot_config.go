package aiprovider

import (
	"os/exec"
	"os"
	"strings"
)

// CopilotConfig configures the Copilot SDK client.
type CopilotConfig struct {
	CLIPath         string
	LogLevel        string
	WorkingDir      string
	GitHubToken     string
	UseLoggedInUser bool
	Model           string
	ReasoningEffort string
	PermissionMode  string
	AutoUserAnswer  string
	Streaming       bool
}

// DefaultCopilotConfig returns sensible defaults for programmatic Copilot use.
func DefaultCopilotConfig(workingDir string) CopilotConfig {
	return CopilotConfig{
		LogLevel:        "error",
		WorkingDir:      workingDir,
		UseLoggedInUser: true,
		Model:           "gpt-5",
		ReasoningEffort: "medium",
		PermissionMode:  string(ModeAllowCopilotText),
		AutoUserAnswer:  "继续执行",
	}
}

// ModeAllowCopilotText is the default permission mode for text-only completions.
const ModeAllowCopilotText = "allow"

// CopilotCLIExists reports whether a Copilot CLI binary appears available.
func CopilotCLIExists(cliPath string) bool {
	cliPath = strings.TrimSpace(cliPath)
	if cliPath != "" {
		if _, err := exec.LookPath(cliPath); err == nil {
			return true
		}
		if _, err := os.Stat(cliPath); err == nil {
			return true
		}
		return false
	}
	for _, name := range []string{"copilot", "github-copilot-cli"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}
