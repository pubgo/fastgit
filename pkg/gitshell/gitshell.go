package gitshell

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

// RunInDir executes git command in the provided directory and returns trimmed stdout.
func RunInDir(dir string, args ...string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("empty start dir")
	}
	if len(args) == 0 {
		return "", errors.New("empty git args")
	}

	if _, err := exec.LookPath("git"); err != nil {
		return "", err
	}

	cmdArgs := make([]string, 0, len(args)+2)
	cmdArgs = append(cmdArgs, "-C", dir)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("git", cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

// DetectBranch returns current branch name when available.
// In detached HEAD mode, it returns "detached@<short_sha>".
// For non-repository directories, it returns an empty string.
func DetectBranch(startDir string) string {
	startDir = strings.TrimSpace(startDir)
	if startDir == "" {
		return ""
	}

	branch, err := RunInDir(startDir, "branch", "--show-current")
	if err == nil && branch != "" {
		return branch
	}

	head, err := RunInDir(startDir, "rev-parse", "--short=12", "HEAD")
	if err != nil || head == "" {
		return ""
	}

	return "detached@" + head
}

// IsDirty reports whether the git working tree contains uncommitted changes.
// For non-repository directories, it returns false.
func IsDirty(startDir string) bool {
	startDir = strings.TrimSpace(startDir)
	if startDir == "" {
		return false
	}

	output, err := RunInDir(startDir, "status", "--porcelain")
	if err != nil {
		return false
	}

	return strings.TrimSpace(output) != ""
}
