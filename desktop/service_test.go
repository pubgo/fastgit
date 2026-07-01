package main

import (
	"os"
	"path/filepath"
	"reflect"
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
