package prcmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// RepoContext holds git metadata needed for PR operations.
type RepoContext struct {
	RepoRoot string
	Branch   string
	Upstream string
	BaseRef  string
}

// LoadRepoContext resolves branch, upstream, and base ref for the current repo.
func LoadRepoContext(ctx context.Context, repoRoot string) (RepoContext, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		var err error
		repoRoot, err = gitOutput(ctx, ".", "rev-parse", "--show-toplevel")
		if err != nil {
			return RepoContext{}, err
		}
	}

	branch, err := gitOutput(ctx, repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return RepoContext{}, err
	}
	if branch == "HEAD" {
		return RepoContext{}, fmt.Errorf("detached HEAD; checkout a branch before creating a PR")
	}

	upstream, err := gitOutput(ctx, repoRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil || strings.TrimSpace(upstream) == "" {
		return RepoContext{}, fmt.Errorf("branch %q has no upstream; push with -u first", branch)
	}

	baseRef, err := detectBaseRef(ctx, repoRoot, "")
	if err != nil {
		return RepoContext{}, err
	}

	return RepoContext{
		RepoRoot: repoRoot,
		Branch:   branch,
		Upstream: upstream,
		BaseRef:  baseRef,
	}, nil
}

func detectBaseRef(ctx context.Context, repoRoot, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		if _, err := gitOutput(ctx, repoRoot, "rev-parse", "--verify", requested); err != nil {
			return "", fmt.Errorf("base ref %q not found: %w", requested, err)
		}
		return requested, nil
	}

	candidates := make([]string, 0, 5)
	if output, err := gitOutput(ctx, repoRoot, "rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil {
		branch := strings.TrimSpace(output)
		if branch != "" && branch != "origin/HEAD" {
			candidates = append(candidates, branch)
		}
	}
	candidates = append(candidates, "origin/main", "main", "origin/master", "master")

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if _, err := gitOutput(ctx, repoRoot, "rev-parse", "--verify", candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to detect base ref for repo %s", repoRoot)
}

func gitOutput(ctx context.Context, repoRoot string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", repoRoot}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func baseBranchName(baseRef string) string {
	baseRef = strings.TrimSpace(baseRef)
	if strings.Contains(baseRef, "/") {
		parts := strings.SplitN(baseRef, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return baseRef
}
