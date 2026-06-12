package chglogcmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureChangelogScaffoldCreatesTemplates(t *testing.T) {
	repo := t.TempDir()

	result, err := ensureChangelogScaffold(repo, scaffoldOptions{
		Version:                "v1.2.3",
		CreateVersionIfMissing: true,
	})
	if err != nil {
		t.Fatalf("ensureChangelogScaffold() error = %v", err)
	}
	if len(result.Created) == 0 {
		t.Fatalf("expected scaffold to create files")
	}

	paths := buildPaths(repo)
	assertFileContains(t, paths.VersionFile, "v1.2.3")
	assertFileContains(t, paths.UnreleasedFile, "# [Unreleased]")
	assertFileContains(t, paths.ReadmeFile, "Changelog 索引")
	assertFileContains(t, paths.ReadmeFile, "Unreleased.md")
	assertFileContains(t, paths.ChangelogPromptFile, "name: changelog")
	assertFileContains(t, paths.ChangelogRulesFile, "applyTo: \".version/changelog/*.md\"")
	assertFileContains(t, paths.ReleaseRulesFile, "发布前核对规则")
}

func TestReleaseChangelogCreatesVersionFileAndResetsTemplate(t *testing.T) {
	repo := t.TempDir()
	paths := buildPaths(repo)
	if _, err := ensureChangelogScaffold(repo, scaffoldOptions{Version: "v0.4.0", CreateVersionIfMissing: true}); err != nil {
		t.Fatalf("ensureChangelogScaffold() error = %v", err)
	}

	unreleased := `# [Unreleased]

## 新增

- 新增 changelog draft 子命令

## 修复

暂无

## 变更

- 调整 release 工作流

## 文档

- 补充使用说明
`
	if err := os.WriteFile(paths.UnreleasedFile, []byte(unreleased), 0o644); err != nil {
		t.Fatalf("write unreleased: %v", err)
	}

	result, err := releaseChangelog(repo, releaseOptions{Version: "v0.4.0", Bump: "minor"})
	if err != nil {
		t.Fatalf("releaseChangelog() error = %v", err)
	}
	if result.NextVersion != "v0.5.0" {
		t.Fatalf("expected next version v0.5.0, got %s", result.NextVersion)
	}
	if len(result.CreatedFiles) != 1 || result.CreatedFiles[0] != filepath.Join(paths.ChangelogDir, "v0.4.0.md") {
		t.Fatalf("expected created release file, got %+v", result.CreatedFiles)
	}

	releaseFile := filepath.Join(paths.ChangelogDir, "v0.4.0.md")
	assertFileContains(t, releaseFile, "# [v0.4.0] - ")
	assertFileContains(t, releaseFile, "- 新增 changelog draft 子命令")
	assertFileContains(t, paths.UnreleasedFile, "# [Unreleased]")
	assertFileContains(t, paths.UnreleasedFile, "暂无")
	assertFileContains(t, paths.ReadmeFile, "[`v0.4.0.md`](v0.4.0.md)")
	assertFileContains(t, paths.VersionFile, "v0.5.0")
}

func TestReleaseChangelogRejectsEmptyUnreleased(t *testing.T) {
	repo := t.TempDir()
	if _, err := ensureChangelogScaffold(repo, scaffoldOptions{Version: "v0.4.0", CreateVersionIfMissing: true}); err != nil {
		t.Fatalf("ensureChangelogScaffold() error = %v", err)
	}

	_, err := releaseChangelog(repo, releaseOptions{Version: "v0.4.0"})
	if err == nil {
		t.Fatalf("expected empty unreleased to be rejected")
	}
	if !strings.Contains(err.Error(), "没有可发布的变更条目") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildDraftPromptIncludesBaseAndDiff(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "feat: init")
	runGit(t, repo, "branch", "-M", "main")

	if _, err := ensureChangelogScaffold(repo, scaffoldOptions{Version: "v0.1.0", CreateVersionIfMissing: true}); err != nil {
		t.Fatalf("ensureChangelogScaffold() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("rewrite README: %v", err)
	}

	prompt, base, err := buildDraftPrompt(context.Background(), repo, "main")
	if err != nil {
		t.Fatalf("buildDraftPrompt() error = %v", err)
	}
	if base != "main" {
		t.Fatalf("expected base main, got %s", base)
	}
	if !strings.Contains(prompt, "git diff main --stat") {
		t.Fatalf("prompt missing diff stat heading: %s", prompt)
	}
	if !strings.Contains(prompt, "README.md") {
		t.Fatalf("prompt missing changed file list: %s", prompt)
	}
	if !strings.Contains(prompt, "只允许修改 .version/changelog/Unreleased.md") {
		t.Fatalf("prompt missing target-file restriction: %s", prompt)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("expected %s to contain %q, got:\n%s", path, want, string(content))
	}
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}
