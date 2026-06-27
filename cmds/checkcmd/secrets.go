package checkcmd

import (
	"os/exec"
	"strings"
)

// resolveSecretScanCommand picks gitleaks or trufflehog when available.
func resolveSecretScanCommand(stagedOnly bool) (command string, tool string) {
	if path, err := exec.LookPath("gitleaks"); err == nil && strings.TrimSpace(path) != "" {
		if stagedOnly {
			return "gitleaks protect --staged --redact --no-banner", "gitleaks"
		}
		return "gitleaks detect --source . --redact --no-banner", "gitleaks"
	}
	if path, err := exec.LookPath("trufflehog"); err == nil && strings.TrimSpace(path) != "" {
		if stagedOnly {
			return "trufflehog git --staged --no-update", "trufflehog"
		}
		return "trufflehog git file://. --no-update", "trufflehog"
	}
	return "", ""
}
