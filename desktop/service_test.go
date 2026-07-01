package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseGitHubRemote(t *testing.T) {
	cases := []struct {
		in    string
		owner string
		repo  string
	}{
		{"git@github.com:pubgo/fastgit.git", "pubgo", "fastgit"},
		{"https://github.com/pubgo/fastgit.git", "pubgo", "fastgit"},
		{"ssh://git@github.com/pubgo/fastgit", "pubgo", "fastgit"},
	}

	for _, c := range cases {
		owner, repo, err := parseGitHubRemote(c.in)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", c.in, err)
		}
		if owner != c.owner || repo != c.repo {
			t.Fatalf("parse mismatch for %s: got %s/%s want %s/%s", c.in, owner, repo, c.owner, c.repo)
		}
	}
}

func TestDetermineWorktreeNames(t *testing.T) {
	b, d := determineWorktreeNames("123")
	if b != "123/impl" || d != "123" {
		t.Fatalf("unexpected simple mapping: %s %s", b, d)
	}
	b, d = determineWorktreeNames("feature/a")
	if b != "feature/a" || d != "feature-a" {
		t.Fatalf("unexpected branch mapping: %s %s", b, d)
	}
}

func TestNormalizeSSHPaths(t *testing.T) {
	got := normalizeSSHPaths([]string{
		"~/.ssh/id_ed25519 ~/.ssh/id_rsa",
		"'/tmp/custom key'",
		"\"~/quoted\"",
	}, "/Users/tester")

	want := []string{
		"/Users/tester/.ssh/id_ed25519",
		"/Users/tester/.ssh/id_rsa",
		"/tmp/custom key",
		"/Users/tester/quoted",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeSSHPaths mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestExistingFilesIgnoresMissingEntries(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(present, []byte("github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := existingFiles([]string{
		filepath.Join(dir, "missing_known_hosts2"),
		present,
	})
	if err != nil {
		t.Fatalf("existingFiles returned error: %v", err)
	}

	want := []string{present}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("existingFiles mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestBranchForceSync(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	local := filepath.Join(root, "local")

	runGit(t, root, "init", "--bare", remote)
	runGit(t, root, "clone", remote, seed)
	runGit(t, seed, "config", "user.name", "Fastgit Test")
	runGit(t, seed, "config", "user.email", "fastgit@example.com")
	runGit(t, seed, "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", "README.md")
	runGit(t, seed, "commit", "-m", "seed v1")
	runGit(t, seed, "push", "-u", "origin", "main")

	runGit(t, root, "clone", "--branch", "main", remote, local)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "commit", "-am", "seed v2")
	runGit(t, seed, "push", "origin", "main")

	if err := os.WriteFile(filepath.Join(local, "README.md"), []byte("local dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "scratch.txt"), []byte("temp\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := &FastgitService{repoRoot: local}
	out, err := svc.branchForceSync(context.Background(), "main")
	if err != nil {
		t.Fatalf("branchForceSync returned error: %v", err)
	}
	if !strings.Contains(out, "force aligned: main -> origin/main") {
		t.Fatalf("unexpected output: %s", out)
	}

	readme, err := os.ReadFile(filepath.Join(local, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(readme) != "seed v2\n" {
		t.Fatalf("README content mismatch: %q", string(readme))
	}
	if _, err := os.Stat(filepath.Join(local, "scratch.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected scratch.txt to be removed, got err=%v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}
