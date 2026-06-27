package prcmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GhClient wraps the GitHub CLI.
type GhClient struct {
	repoRoot string
}

// NewGhClient creates a gh client scoped to a repository directory.
func NewGhClient(repoRoot string) *GhClient {
	return &GhClient{repoRoot: repoRoot}
}

// EnsureAvailable verifies gh is installed and authenticated enough to run.
func (g *GhClient) EnsureAvailable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	cmd.Dir = g.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh CLI not ready: %w\n%s\nInstall: https://cli.github.com/", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CreatePR opens a pull request on GitHub.
func (g *GhClient) CreatePR(ctx context.Context, draft Draft) (string, error) {
	args := []string{
		"pr", "create",
		"--title", draft.Title,
		"--body", draft.Body,
		"--base", draft.Base,
		"--head", draft.Head,
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = g.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ViewPR returns gh pr view output for the current branch.
func (g *GhClient) ViewPR(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", "--json", "title,state,url,reviewDecision,statusCheckRollup", "--jq", "{title,state,url,reviewDecision,checks:(.statusCheckRollup|length)}")
	cmd.Dir = g.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr view: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// EditPR updates the current branch pull request title and body.
func (g *GhClient) EditPR(ctx context.Context, draft Draft) error {
	cmd := exec.CommandContext(ctx, "gh", "pr", "edit", "--title", draft.Title, "--body", draft.Body)
	cmd.Dir = g.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr edit: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SyncPR rebases onto base and force-pushes with lease (when not dry-run).
func (g *GhClient) SyncPR(ctx context.Context, rc RepoContext, dryRun bool) error {
	if dryRun {
		return nil
	}
	if err := execInRepo(ctx, rc.RepoRoot, "git", "fetch", "origin", rc.BaseRef); err != nil {
		return err
	}
	if err := execInRepo(ctx, rc.RepoRoot, "git", "rebase", rc.BaseRef); err != nil {
		return fmt.Errorf("rebase onto %s: %w", rc.BaseRef, err)
	}
	return execInRepo(ctx, rc.RepoRoot, "git", "push", "--force-with-lease")
}

// MergePR merges the current branch PR.
func (g *GhClient) MergePR(ctx context.Context, method string, dryRun bool) (string, error) {
	method = strings.TrimSpace(method)
	if method == "" {
		method = "squash"
	}
	if dryRun {
		return fmt.Sprintf("would merge with --%s", method), nil
	}
	cmd := exec.CommandContext(ctx, "gh", "pr", "merge", "--"+method)
	cmd.Dir = g.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr merge: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func execInRepo(ctx context.Context, repoRoot string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
