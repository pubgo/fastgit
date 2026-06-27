package chglogcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pubgo/fastgit/pkg/aiprovider"
	"github.com/pubgo/fastgit/pkg/copilotperm"
	"github.com/pubgo/redant"
)

type draftCopilotOptions struct {
	CLIPath         string
	LogLevel        string
	WorkingDir      string
	GitHubToken     string
	UseLoggedInUser bool
	Model           string
	ReasoningEffort string
	Streaming       bool
	AutoUserAnswer  string
	PermissionMode  string
}

func buildDraftPrompt(ctx context.Context, repoRoot, requestedBase string) (string, string, error) {
	paths := buildPaths(repoRoot)
	baseRef, err := detectBaseRef(ctx, repoRoot, requestedBase)
	if err != nil {
		return "", "", err
	}

	versionContent, err := os.ReadFile(paths.VersionFile)
	if err != nil {
		return "", "", fmt.Errorf("read VERSION: %w", err)
	}
	unreleasedContent, err := os.ReadFile(paths.UnreleasedFile)
	if err != nil {
		return "", "", fmt.Errorf("read Unreleased.md: %w", err)
	}
	readmeContent, err := os.ReadFile(paths.ReadmeFile)
	if err != nil {
		return "", "", fmt.Errorf("read changelog README.md: %w", err)
	}

	diffStat, err := gitOutput(ctx, repoRoot, "diff", baseRef, "--stat")
	if err != nil {
		return "", "", err
	}
	diffNames, err := gitOutput(ctx, repoRoot, "diff", baseRef, "--name-only")
	if err != nil {
		return "", "", err
	}

	prompt := renderDraftPrompt(draftPromptData{
		RepoRoot:          repoRoot,
		BaseRef:           baseRef,
		Version:           strings.TrimSpace(string(versionContent)),
		UnreleasedContent: strings.TrimSpace(string(unreleasedContent)),
		ReadmeContent:     strings.TrimSpace(string(readmeContent)),
		DiffStat:          emptyAsNone(diffStat),
		DiffNames:         emptyAsNone(diffNames),
	})

	return prompt + "\n", baseRef, nil
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

func emptyAsNone(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "(no changes)"
	}
	return text
}

func runDraftWithCopilot(ctx context.Context, inv *redant.Invocation, prompt string, opts draftCopilotOptions) error {
	cfg := aiprovider.CopilotConfig{
		CLIPath:         strings.TrimSpace(opts.CLIPath),
		LogLevel:        defaultString(strings.TrimSpace(opts.LogLevel), "error"),
		WorkingDir:      strings.TrimSpace(opts.WorkingDir),
		GitHubToken:     strings.TrimSpace(opts.GitHubToken),
		UseLoggedInUser: opts.UseLoggedInUser,
		Model:           defaultString(strings.TrimSpace(opts.Model), "gpt-5"),
		ReasoningEffort: defaultString(strings.TrimSpace(opts.ReasoningEffort), "medium"),
		PermissionMode:  defaultString(strings.TrimSpace(opts.PermissionMode), string(copilotperm.ModeDeny)),
		AutoUserAnswer:  defaultString(strings.TrimSpace(opts.AutoUserAnswer), "继续执行"),
		Streaming:       opts.Streaming,
	}

	err := aiprovider.RunCopilotSession(ctx, cfg, prompt, aiprovider.SessionCallbacks{Stdout: inv.Stdout})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(inv.Stdout, "draft completed: .version/changelog/Unreleased.md")
	return nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
