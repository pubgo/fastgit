package gitshell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectBranch_WithGitCLIRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	tmp := t.TempDir()
	runGitForTest(t, tmp, "init")
	runGitForTest(t, tmp, "checkout", "-b", "feat/ctx")

	got := DetectBranch(tmp)
	if got != "feat/ctx" {
		t.Fatalf("expected feat/ctx, got %q", got)
	}

	nested := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested failed: %v", err)
	}
	got = DetectBranch(nested)
	if got != "feat/ctx" {
		t.Fatalf("expected nested path detect feat/ctx, got %q", got)
	}
}

func TestDetectBranch_DetachedHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	tmp := t.TempDir()
	runGitForTest(t, tmp, "init")
	runGitForTest(t, tmp, "config", "user.email", "gitshell-test@example.com")
	runGitForTest(t, tmp, "config", "user.name", "gitshell-test")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README failed: %v", err)
	}
	runGitForTest(t, tmp, "add", "README.md")
	runGitForTest(t, tmp, "commit", "-m", "init")
	runGitForTest(t, tmp, "checkout", "--detach")

	got := DetectBranch(tmp)
	if !strings.HasPrefix(got, "detached@") {
		t.Fatalf("expected detached@ prefix, got %q", got)
	}
}

func TestDetectBranch_NotRepo(t *testing.T) {
	tmp := t.TempDir()
	got := DetectBranch(tmp)
	if got != "" {
		t.Fatalf("expected empty branch for non-repo path, got %q", got)
	}
}

func TestRunInDir_EmptyArgs(t *testing.T) {
	if _, err := RunInDir(t.TempDir()); err == nil {
		t.Fatalf("expected error when args are empty")
	}
}

func TestIsDirty_CleanRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	tmp := t.TempDir()
	runGitForTest(t, tmp, "init")
	runGitForTest(t, tmp, "config", "user.email", "gitshell-test@example.com")
	runGitForTest(t, tmp, "config", "user.name", "gitshell-test")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README failed: %v", err)
	}
	runGitForTest(t, tmp, "add", "README.md")
	runGitForTest(t, tmp, "commit", "-m", "init")

	if IsDirty(tmp) {
		t.Fatalf("expected clean repo to be not dirty")
	}
}

func TestIsDirty_WithWorkingTreeChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	tmp := t.TempDir()
	runGitForTest(t, tmp, "init")
	runGitForTest(t, tmp, "config", "user.email", "gitshell-test@example.com")
	runGitForTest(t, tmp, "config", "user.name", "gitshell-test")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README failed: %v", err)
	}
	runGitForTest(t, tmp, "add", "README.md")
	runGitForTest(t, tmp, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("update README failed: %v", err)
	}

	if !IsDirty(tmp) {
		t.Fatalf("expected repo with working tree changes to be dirty")
	}
}

func TestIsDirty_NotRepo(t *testing.T) {
	if IsDirty(t.TempDir()) {
		t.Fatalf("expected non-repo path to be not dirty")
	}
}

func runGitForTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmdArgs := make([]string, 0, len(args)+2)
	cmdArgs = append(cmdArgs, "-C", dir)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v, output=%s", args, err, strings.TrimSpace(string(out)))
	}
}
