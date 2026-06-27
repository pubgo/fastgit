package checkcmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirstToken(t *testing.T) {
	require.Equal(t, "gofmt", firstToken("gofmt -l ."))
	require.Equal(t, "golangci-lint", firstToken("golangci-lint run ./..."))
	require.Equal(t, "", firstToken(""))
}

func TestStagedFmtCommand(t *testing.T) {
	t.Run("no go files", func(t *testing.T) {
		require.Empty(t, stagedFmtCommand([]string{"README.md", "Makefile"}, false))
	})

	t.Run("list staged go files", func(t *testing.T) {
		cmd := stagedFmtCommand([]string{"a.go", "pkg/b.go", "c.txt"}, false)
		require.Equal(t, "gofmt -l a.go pkg/b.go", cmd)
	})

	t.Run("fix staged go files", func(t *testing.T) {
		cmd := stagedFmtCommand([]string{"main.go"}, true)
		require.Equal(t, "gofmt -w main.go", cmd)
	})
}

func TestDefaultConfigHasExpectedSteps(t *testing.T) {
	cfg := DefaultConfig()
	names := make([]string, 0, len(cfg.Steps))
	for _, step := range cfg.Steps {
		names = append(names, step.Name)
	}
	require.Equal(t, []string{"fmt", "vet", "test", "lint", "secrets"}, names)
}

func TestRunDryRunDoesNotFailOnOptionalMissing(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	results, err := Run(context.Background(), DefaultConfig(), RunOptions{
		DryRun:   true,
		RepoRoot: repo,
	})
	require.NoError(t, err)
	require.NotEmpty(t, results)

	var sawDryRun bool
	for _, r := range results {
		if r.Output != "" && contains(r.Output, "[dry-run]") {
			sawDryRun = true
		}
	}
	require.True(t, sawDryRun)
}

func TestRunStagedOnlyEmptyFails(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	_, err := Run(context.Background(), DefaultConfig(), RunOptions{
		StagedOnly: true,
		RepoRoot:   repo,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no staged files")
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestResolveGitDirFromTempRepo(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repo))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	gitDir, err := resolveGitDir()
	require.NoError(t, err)
	require.NotEmpty(t, gitDir)
	require.True(t, filepath.IsAbs(gitDir))
}
