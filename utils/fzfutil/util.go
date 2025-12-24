package fzfutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	//fzf "github.com/junegunn/fzf/src"
	_ "github.com/junegunn/fzf/src"
)

// isFzfAvailable checks if fzf is available in PATH
func isFzfAvailable() bool {
	_, err := exec.LookPath("fzf")
	return err == nil
}

func SelectWithFzf(ctx context.Context, input io.Reader) (string, error) {
	// Check if fzf is available
	if !isFzfAvailable() {
		return "", fmt.Errorf("fzf not available")
	}

	// Run fzf
	cmd := exec.CommandContext(ctx, "fzf",
		"--height", "40%",
		"--reverse",
		"--border",
		"--prompt", "Select: ",
		//"--header", "Press ESC to cancel",
		"--ansi", // Enable color support
	)

	cmd.Stdin = input
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	selected := strings.TrimSpace(string(output))
	if selected == "" {
		return "", fmt.Errorf("no context selected")
	}

	// Extract context name (remove prefix)
	contextName := strings.TrimSpace(strings.TrimPrefix(selected, "*"))
	contextName = strings.TrimSpace(contextName)

	return contextName, nil
}
