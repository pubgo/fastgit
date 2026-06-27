package copilotperm

import (
	"os"
	"strings"
)

const permissionModeEnv = "FASTGIT_COPILOT_PERMISSION_MODE"

// ResolveMode picks Copilot permission mode.
// Priority: CLI flag > FASTGIT_COPILOT_PERMISSION_MODE > ~/.config/fastgit/config.yaml > commandDefault.
func ResolveMode(cliFlag string, commandDefault Mode) (Mode, error) {
	if raw := strings.TrimSpace(cliFlag); raw != "" {
		return ParseMode(raw)
	}
	if raw := strings.TrimSpace(os.Getenv(permissionModeEnv)); raw != "" {
		return ParseMode(raw)
	}
	if raw := loadGlobalMode(); raw != "" {
		return ParseMode(raw)
	}
	if commandDefault != "" {
		return commandDefault, nil
	}
	return DefaultMode, nil
}
