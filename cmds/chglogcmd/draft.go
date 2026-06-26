package chglogcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	copilot "github.com/github/copilot-sdk/go"
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
	client := copilot.NewClient(&copilot.ClientOptions{
		CLIPath:         strings.TrimSpace(opts.CLIPath),
		LogLevel:        defaultString(strings.TrimSpace(opts.LogLevel), "error"),
		Cwd:             strings.TrimSpace(opts.WorkingDir),
		GitHubToken:     strings.TrimSpace(opts.GitHubToken),
		UseLoggedInUser: copilot.Bool(opts.UseLoggedInUser),
		AutoStart:       copilot.Bool(false),
	})

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("start copilot client: %w", err)
	}
	defer func() { _ = client.Stop() }()

	session, err := client.CreateSession(ctx, &copilot.SessionConfig{
		Model:               defaultString(strings.TrimSpace(opts.Model), "gpt-5"),
		ReasoningEffort:     defaultString(strings.TrimSpace(opts.ReasoningEffort), "medium"),
		WorkingDirectory:    strings.TrimSpace(opts.WorkingDir),
		Streaming:           opts.Streaming,
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
		OnUserInputRequest: func(request copilot.UserInputRequest, invocation copilot.UserInputInvocation) (copilot.UserInputResponse, error) {
			_, _ = fmt.Fprintf(inv.Stdout, "[ask_user] session=%s question=%s\n", invocation.SessionID, request.Question)
			answer := defaultString(strings.TrimSpace(opts.AutoUserAnswer), "继续执行")
			return copilot.UserInputResponse{Answer: answer, WasFreeform: true}, nil
		},
	})
	if err != nil {
		return fmt.Errorf("create copilot session: %w", err)
	}
	defer func() { _ = session.Disconnect() }()

	_, _ = fmt.Fprintf(inv.Stdout, "session=%s\n", session.SessionID)

	done := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	unsub := session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case "assistant.message_delta", "assistant.reasoning_delta":
			if opts.Streaming && event.Data.DeltaContent != nil {
				_, _ = fmt.Fprint(inv.Stdout, *event.Data.DeltaContent)
			}
		case "assistant.message":
			if event.Data.Content != nil {
				if opts.Streaming {
					_, _ = fmt.Fprintln(inv.Stdout)
				}
				_, _ = fmt.Fprintf(inv.Stdout, "assistant: %s\n", *event.Data.Content)
			}
		case "session.error":
			if event.Data.Message != nil {
				select {
				case errCh <- fmt.Errorf("session error: %s", *event.Data.Message):
				default:
				}
			}
		case "session.idle":
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	defer unsub()

	if _, err := session.Send(ctx, copilot.MessageOptions{Prompt: prompt}); err != nil {
		return fmt.Errorf("send draft prompt: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	select {
	case <-done:
		_, _ = fmt.Fprintln(inv.Stdout, "draft completed: .version/changelog/Unreleased.md")
		return nil
	case err := <-errCh:
		return err
	case <-waitCtx.Done():
		return fmt.Errorf("wait session idle: %w", waitCtx.Err())
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
