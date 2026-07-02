package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v6"
	gitconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
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

func TestValidateForceSyncConfirmation(t *testing.T) {
	if err := validateForceSyncConfirmation("RESET"); err != nil {
		t.Fatalf("expected RESET to pass, got %v", err)
	}
	if err := validateForceSyncConfirmation(" reset "); err != nil {
		t.Fatalf("expected case-insensitive RESET to pass, got %v", err)
	}
	if err := validateForceSyncConfirmation("DELETE"); err == nil {
		t.Fatal("expected invalid confirmation to fail")
	}
}

func TestDefaultRemoteName(t *testing.T) {
	cfg := &gitconfig.Config{
		Remotes: map[string]*gitconfig.RemoteConfig{
			"origin": {Name: "origin"},
			"backup": {Name: "backup"},
		},
		Branches: map[string]*gitconfig.Branch{
			"main": {Name: "main", Remote: "backup"},
		},
	}

	if got := defaultRemoteName(cfg, "main"); got != "backup" {
		t.Fatalf("expected branch tracking remote backup, got %q", got)
	}
	if got := defaultRemoteName(cfg, "feature/no-track"); got != "origin" {
		t.Fatalf("expected origin fallback, got %q", got)
	}

	cfg = &gitconfig.Config{
		Remotes: map[string]*gitconfig.RemoteConfig{
			"backup": {Name: "backup"},
			"mirror": {Name: "mirror"},
		},
	}
	if got := defaultRemoteName(cfg, ""); got != "backup" {
		t.Fatalf("expected alphabetical fallback backup, got %q", got)
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
	out, err := svc.branchForceSync(context.Background(), "origin", "main")
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

func TestBranchListIncludesTrackingStatus(t *testing.T) {
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
	runGit(t, local, "config", "user.name", "Fastgit Test")
	runGit(t, local, "config", "user.email", "fastgit@example.com")
	if err := os.WriteFile(filepath.Join(local, "README.md"), []byte("local ahead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, local, "commit", "-am", "local ahead")
	runGit(t, local, "branch", "local-only")

	svc := &FastgitService{repoRoot: local}
	out, err := svc.branchList(context.Background())
	if err != nil {
		t.Fatalf("branchList returned error: %v", err)
	}
	if !strings.Contains(out, "local-only\tlocal\t\t\tno-upstream\t0\t0") {
		t.Fatalf("expected local-only branch in output, got: %s", out)
	}
	if !strings.Contains(out, "main\tcurrent\torigin\torigin/main\tahead\t1\t0") {
		t.Fatalf("expected tracked main branch in output, got: %s", out)
	}
}

func TestTagForceSync(t *testing.T) {
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
	runGit(t, seed, "tag", "v1.0.0")
	runGit(t, seed, "push", "-u", "origin", "main")
	runGit(t, seed, "push", "origin", "v1.0.0")

	runGit(t, root, "clone", "--branch", "main", remote, local)

	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "commit", "-am", "seed v2")
	runGit(t, seed, "tag", "-f", "v1.0.0")
	runGit(t, seed, "push", "origin", "main")
	runGit(t, seed, "push", "--force", "origin", "v1.0.0")

	if err := os.WriteFile(filepath.Join(local, "LOCAL.md"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, local, "add", "LOCAL.md")
	runGit(t, local, "commit", "-m", "local only")
	runGit(t, local, "tag", "-f", "v1.0.0")

	svc := &FastgitService{repoRoot: local}
	out, err := svc.tagForceSync(context.Background(), "origin", "v1.0.0")
	if err != nil {
		t.Fatalf("tagForceSync returned error: %v", err)
	}
	if !strings.Contains(out, "force aligned tag: v1.0.0") {
		t.Fatalf("unexpected output: %s", out)
	}

	localRepo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	localHash, err := svc.localTagHash(localRepo, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	seedRepo, err := git.PlainOpen(seed)
	if err != nil {
		t.Fatal(err)
	}
	seedHash, err := svc.localTagHash(seedRepo, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if localHash != seedHash {
		t.Fatalf("tag hash mismatch: local=%s remote-seed=%s", localHash, seedHash)
	}
}

func TestRemoteLifecycle(t *testing.T) {
	root := t.TempDir()
	remoteA := filepath.Join(root, "remote-a.git")
	remoteB := filepath.Join(root, "remote-b.git")
	seed := filepath.Join(root, "seed")
	local := filepath.Join(root, "local")

	runGit(t, root, "init", "--bare", remoteA)
	runGit(t, root, "init", "--bare", remoteB)
	runGit(t, root, "clone", remoteA, seed)
	runGit(t, seed, "config", "user.name", "Fastgit Test")
	runGit(t, seed, "config", "user.email", "fastgit@example.com")
	runGit(t, seed, "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", "README.md")
	runGit(t, seed, "commit", "-m", "seed remote")
	runGit(t, seed, "push", "-u", "origin", "main")
	runGit(t, seed, "remote", "add", "backup", remoteB)
	runGit(t, seed, "push", "-u", "backup", "main")

	runGit(t, root, "init", local)
	svc := &FastgitService{repoRoot: local}

	if out, err := svc.remoteAdd(context.Background(), "origin", remoteA, remoteB); err != nil {
		t.Fatalf("remoteAdd returned error: %v", err)
	} else if !strings.Contains(out, "remote added: origin") {
		t.Fatalf("unexpected remoteAdd output: %s", out)
	}
	if out, err := svc.remoteAdd(context.Background(), "backup", remoteB, ""); err != nil {
		t.Fatalf("remoteAdd backup returned error: %v", err)
	} else if !strings.Contains(out, "remote added: backup") {
		t.Fatalf("unexpected backup remoteAdd output: %s", out)
	}

	list, err := svc.remoteList(context.Background())
	if err != nil {
		t.Fatalf("remoteList returned error: %v", err)
	}
	if !strings.Contains(list, "origin\tfile\t") || !strings.Contains(list, "backup\tfile\t") {
		t.Fatalf("unexpected remoteList output: %s", list)
	}

	if out, err := svc.remoteFetchAll(context.Background()); err != nil {
		t.Fatalf("remoteFetchAll returned error: %v", err)
	} else if !strings.Contains(out, "origin") || !strings.Contains(out, "backup") {
		t.Fatalf("unexpected remoteFetchAll output: %s", out)
	}

	localRepo, err := git.PlainOpen(local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localRepo.Reference(plumbing.ReferenceName("refs/remotes/origin/main"), true); err != nil {
		t.Fatalf("expected fetched remote branch, got %v", err)
	}
	if _, err := localRepo.Reference(plumbing.ReferenceName("refs/remotes/backup/main"), true); err != nil {
		t.Fatalf("expected fetched backup remote branch, got %v", err)
	}

	if out, err := svc.remoteSetPushURL(context.Background(), "origin", remoteB); err != nil {
		t.Fatalf("remoteSetPushURL returned error: %v", err)
	} else if !strings.Contains(out, "remote push URL updated: origin") {
		t.Fatalf("unexpected remoteSetPushURL output: %s", out)
	}

	list, err = svc.remoteList(context.Background())
	if err != nil {
		t.Fatalf("remoteList after push URL update returned error: %v", err)
	}
	if !strings.Contains(list, remoteB) {
		t.Fatalf("expected push URL in list, got: %s", list)
	}

	if out, err := svc.remoteUpdate(context.Background(), "origin", remoteA, ""); err != nil {
		t.Fatalf("remoteUpdate returned error: %v", err)
	} else if !strings.Contains(out, "push follows fetch") {
		t.Fatalf("unexpected remoteUpdate output: %s", out)
	}

	list, err = svc.remoteList(context.Background())
	if err != nil {
		t.Fatalf("remoteList after remoteUpdate returned error: %v", err)
	}
	if !strings.Contains(list, "origin\tfile\t"+remoteA+"\t"+remoteA+"\tdefault") {
		t.Fatalf("expected cleared push URL to fall back to fetch URL, got: %s", list)
	}

	if out, err := svc.remoteRename(context.Background(), "origin", "upstream"); err != nil {
		t.Fatalf("remoteRename returned error: %v", err)
	} else if !strings.Contains(out, "remote renamed: origin -> upstream") {
		t.Fatalf("unexpected remoteRename output: %s", out)
	}

	list, err = svc.remoteList(context.Background())
	if err != nil {
		t.Fatalf("remoteList after rename returned error: %v", err)
	}
	if !strings.Contains(list, "upstream\tfile\t") {
		t.Fatalf("expected renamed remote in list, got: %s", list)
	}

	if out, err := svc.remoteSetURL(context.Background(), "upstream", remoteB); err != nil {
		t.Fatalf("remoteSetURL returned error: %v", err)
	} else if !strings.Contains(out, "remote updated: upstream") {
		t.Fatalf("unexpected remoteSetURL output: %s", out)
	}

	list, err = svc.remoteList(context.Background())
	if err != nil {
		t.Fatalf("remoteList after fetch URL update returned error: %v", err)
	}
	if !strings.Contains(list, "upstream\tfile\t"+remoteB) {
		t.Fatalf("expected updated fetch URL in list, got: %s", list)
	}

	if out, err := svc.remoteRemove(context.Background(), "upstream"); err != nil {
		t.Fatalf("remoteRemove returned error: %v", err)
	} else if !strings.Contains(out, "remote removed: upstream") {
		t.Fatalf("unexpected remoteRemove output: %s", out)
	}

	if out, err := svc.remoteRemove(context.Background(), "backup"); err != nil {
		t.Fatalf("remoteRemove backup returned error: %v", err)
	} else if !strings.Contains(out, "remote removed: backup") {
		t.Fatalf("unexpected backup remoteRemove output: %s", out)
	}

	list, err = svc.remoteList(context.Background())
	if err != nil {
		t.Fatalf("remoteList after remove returned error: %v", err)
	}
	if list != "no remotes" {
		t.Fatalf("expected no remotes, got: %s", list)
	}
}

func TestRepoPathOperations(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.name", "Fastgit Test")
	runGit(t, root, "config", "user.email", "fastgit@example.com")

	trackedPath := filepath.Join(root, "README.md")
	if err := os.WriteFile(trackedPath, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "init")

	if err := os.WriteFile(trackedPath, []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	untrackedPath := filepath.Join(root, "scratch.txt")
	if err := os.WriteFile(untrackedPath, []byte("scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := &FastgitService{repoRoot: root}
	ctx := context.Background()

	if out, err := svc.repoStagePath(ctx, "README.md"); err != nil {
		t.Fatalf("repoStagePath returned error: %v", err)
	} else if !strings.Contains(out, "staged: README.md") {
		t.Fatalf("unexpected repoStagePath output: %s", out)
	}
	if staged := runGit(t, root, "diff", "--cached", "--name-only"); !strings.Contains(staged, "README.md") {
		t.Fatalf("expected README.md in staged diff, got:\n%s", staged)
	}

	if out, err := svc.repoUnstagePath(ctx, "README.md"); err != nil {
		t.Fatalf("repoUnstagePath returned error: %v", err)
	} else if !strings.Contains(out, "unstaged: README.md") {
		t.Fatalf("unexpected repoUnstagePath output: %s", out)
	}
	if staged := runGit(t, root, "diff", "--cached", "--name-only"); strings.Contains(staged, "README.md") {
		t.Fatalf("expected README.md not staged, got staged diff:\n%s", staged)
	}
	if unstaged := runGit(t, root, "diff", "--name-only"); !strings.Contains(unstaged, "README.md") {
		t.Fatalf("expected README.md in unstaged diff, got:\n%s", unstaged)
	}

	if out, err := svc.repoDiscardPath(ctx, "README.md"); err != nil {
		t.Fatalf("repoDiscardPath for tracked file returned error: %v", err)
	} else if !strings.Contains(out, "discarded: README.md") {
		t.Fatalf("unexpected repoDiscardPath output for tracked file: %s", out)
	}
	if content, err := os.ReadFile(trackedPath); err != nil {
		t.Fatal(err)
	} else if string(content) != "v1\n" {
		t.Fatalf("expected README.md restored to v1, got %q", string(content))
	}

	if out, err := svc.repoDiscardPath(ctx, "scratch.txt"); err != nil {
		t.Fatalf("repoDiscardPath for untracked file returned error: %v", err)
	} else if !strings.Contains(out, "untracked removed") {
		t.Fatalf("unexpected repoDiscardPath output for untracked file: %s", out)
	}
	if _, err := os.Stat(untrackedPath); !os.IsNotExist(err) {
		t.Fatalf("expected scratch.txt removed, got err=%v", err)
	}
}

func TestNormalizeRepoRelativePath(t *testing.T) {
	okCases := map[string]string{
		"README.md":      "README.md",
		"./docs/a.md":    "docs/a.md",
		"nested\\b.go":   "nested/b.go",
		"nested/../a.go": "a.go",
	}
	for input, expected := range okCases {
		got, err := normalizeRepoRelativePath(input)
		if err != nil {
			t.Fatalf("normalizeRepoRelativePath(%q) returned error: %v", input, err)
		}
		if got != expected {
			t.Fatalf("normalizeRepoRelativePath(%q) = %q, want %q", input, got, expected)
		}
	}

	badCases := []string{"", " ", ".", "..", "../a", "/tmp/a"}
	for _, input := range badCases {
		if _, err := normalizeRepoRelativePath(input); err == nil {
			t.Fatalf("normalizeRepoRelativePath(%q) expected error", input)
		}
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
