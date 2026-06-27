package prcmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuggestTitleFromCommits(t *testing.T) {
	title := suggestTitle("feature/foo", "- feat: add check command (Barry)")
	require.Equal(t, "feat: add check command", title)
}

func TestSuggestTitleFromBranch(t *testing.T) {
	title := suggestTitle("feature/add-pr-cmd", "")
	require.Contains(t, title, "feature")
}

func TestAssessRiskLargeChangeSet(t *testing.T) {
	names := make([]string, 25)
	for i := range names {
		names[i] = "file" + string(rune('a'+i)) + ".go"
	}
	risks := assessRisk(strings.Join(names, "\n"))
	require.NotEmpty(t, risks)
	require.Contains(t, risks[0], "large change set")
}

func TestAssessRiskSensitivePath(t *testing.T) {
	risks := assessRisk(".env\npkg/auth/login.go")
	found := false
	for _, r := range risks {
		if strings.Contains(r, ".env") || strings.Contains(r, "auth") {
			found = true
		}
	}
	require.True(t, found)
}

func TestBuildDraftInTempRepo(t *testing.T) {
	repo := initTempGitRepo(t)

	writeFile(t, repo, "main.go", "package main\nfunc main() {}\n")
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "feat: initial commit")

	runGit(t, repo, "checkout", "-b", "feature/demo")
	writeFile(t, repo, "main.go", "package main\nfunc main() { println(\"hi\") }\n")
	runGit(t, repo, "add", "main.go")
	runGit(t, repo, "commit", "-m", "feat: greet")

	rc := RepoContext{
		RepoRoot: repo,
		Branch:   "feature/demo",
		BaseRef:  "main",
	}

	draft, err := BuildDraft(context.Background(), rc)
	require.NoError(t, err)
	require.Contains(t, draft.Title, "feat")
	require.Contains(t, draft.Body, "## Summary")
	require.Contains(t, draft.Body, "## Risk")
	require.Contains(t, draft.Body, "## Test plan")
	require.Equal(t, "main", draft.Base)
	require.Equal(t, "feature/demo", draft.Head)
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
