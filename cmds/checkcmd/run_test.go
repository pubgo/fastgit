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

func TestRunMultiPackageStagedDryRun(t *testing.T) {
	repo := setupMultiPackageGoRepo(t)

	results, err := Run(context.Background(), minimalGoConfig(), RunOptions{
		StagedOnly: true,
		DryRun:     true,
		RepoRoot:   repo,
	})
	require.NoError(t, err)

	vet := findStepResult(results, "vet")
	require.NotNil(t, vet)
	require.Contains(t, vet.Output, "./pkg/alpha/...")
	require.Contains(t, vet.Output, "./pkg/beta/...")

	test := findStepResult(results, "test")
	require.NotNil(t, test)
	require.Contains(t, test.Output, "./pkg/alpha/...")
	require.Contains(t, test.Output, "./pkg/beta/...")
}

func TestRunMultiPackageStagedExecutes(t *testing.T) {
	repo := setupMultiPackageGoRepo(t)

	results, err := Run(context.Background(), minimalGoConfig(), RunOptions{
		StagedOnly: true,
		RepoRoot:   repo,
	})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	for _, r := range results {
		require.Nil(t, r.Err, "step %s failed: %s", r.Step.Name, r.Output)
	}
}

func TestRunFailureIncludesStepName(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	cfg := Config{Steps: []Step{{Name: "broken", Command: "exit 1"}}}
	results, err := Run(context.Background(), cfg, RunOptions{RepoRoot: repo})
	require.Error(t, err)
	require.Contains(t, err.Error(), "broken failed")
	require.Equal(t, "broken", FailedStepName(results))
}

func TestRunDryRunDoesNotModifyWorkspace(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	writeRepoFile(t, repo, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeRepoFile(t, repo, "main.go", "package main\n\nfunc main() {\nprintln(\"hi\")\n}\n")
	runGit(t, repo, "add", "go.mod", "main.go")

	before, err := os.ReadFile(filepath.Join(repo, "main.go"))
	require.NoError(t, err)

	_, err = Run(context.Background(), Config{Steps: []Step{
		{Name: "fmt", Command: "gofmt -w .", Fixable: true, FixCommand: "gofmt -w ."},
	}}, RunOptions{DryRun: true, Fix: true, RepoRoot: repo})
	require.NoError(t, err)

	after, err := os.ReadFile(filepath.Join(repo, "main.go"))
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}

func minimalGoConfig() Config {
	return Config{Steps: []Step{
		{Name: "fmt", Command: "gofmt -l .", FixCommand: "gofmt -w .", Fixable: true},
		{Name: "vet", Command: "go vet ./..."},
		{Name: "test", Command: "go test -short ./..."},
	}}
}

func setupMultiPackageGoRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	initGitRepo(t, repo)
	writeRepoFile(t, repo, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeRepoFile(t, repo, "pkg/alpha/a.go", "package alpha\n\nfunc A() int { return 1 }\n")
	writeRepoFile(t, repo, "pkg/beta/b.go", "package beta\n\nfunc B() int { return 2 }\n")
	runGit(t, repo, "add", ".")
	return repo
}

func writeRepoFile(t *testing.T, repo, relPath, content string) {
	t.Helper()
	path := filepath.Join(repo, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func findStepResult(results []StepResult, name string) *StepResult {
	for i := range results {
		if results[i].Step.Name == name {
			return &results[i]
		}
	}
	return nil
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
