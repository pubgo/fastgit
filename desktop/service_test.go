package main

import "testing"

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
