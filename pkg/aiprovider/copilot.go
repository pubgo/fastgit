package aiprovider

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	copilot "github.com/github/copilot-sdk/go"
	"github.com/pubgo/fastgit/pkg/copilotperm"
)

// SessionCallbacks receives Copilot session events.
type SessionCallbacks struct {
	Stdout io.Writer
	OnDelta func(text string)
	OnMessage func(text string)
}

// RunCopilotSession executes a Copilot prompt and waits until the session is idle.
func RunCopilotSession(ctx context.Context, cfg CopilotConfig, prompt string, cb SessionCallbacks) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("copilot prompt is empty")
	}

	client := copilot.NewClient(&copilot.ClientOptions{
		CLIPath:         strings.TrimSpace(cfg.CLIPath),
		LogLevel:        defaultCopilotString(cfg.LogLevel, "error"),
		Cwd:             strings.TrimSpace(cfg.WorkingDir),
		GitHubToken:     strings.TrimSpace(cfg.GitHubToken),
		UseLoggedInUser: copilot.Bool(cfg.UseLoggedInUser),
		AutoStart:       copilot.Bool(false),
	})

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("start copilot client: %w", err)
	}
	defer func() { _ = client.Stop() }()

	auditor, _ := copilotperm.NewFileAuditor()
	mode, err := copilotperm.ResolveMode(cfg.PermissionMode, copilotperm.ModeDeny)
	if err != nil {
		mode = copilotperm.ModeDeny
	}

	stdout := cb.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	prompter := copilotperm.PrompterForMode(mode, copilotperm.DefaultTerminalPrompter(os.Stdin, stdout))

	session, err := client.CreateSession(ctx, &copilot.SessionConfig{
		Model:               defaultCopilotString(cfg.Model, "gpt-5"),
		ReasoningEffort:     defaultCopilotString(cfg.ReasoningEffort, "medium"),
		WorkingDirectory:    strings.TrimSpace(cfg.WorkingDir),
		Streaming:           cfg.Streaming,
		OnPermissionRequest: copilotperm.NewHandler(copilotperm.Options{Mode: mode, Auditor: auditor, Prompter: prompter}),
		OnUserInputRequest: func(request copilot.UserInputRequest, invocation copilot.UserInputInvocation) (copilot.UserInputResponse, error) {
			_, _ = fmt.Fprintf(stdout, "[ask_user] session=%s question=%s\n", invocation.SessionID, request.Question)
			answer := defaultCopilotString(cfg.AutoUserAnswer, "继续执行")
			return copilot.UserInputResponse{Answer: answer, WasFreeform: true}, nil
		},
	})
	if err != nil {
		return fmt.Errorf("create copilot session: %w", err)
	}
	defer func() { _ = session.Disconnect() }()

	done := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	unsub := session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case "assistant.message_delta", "assistant.reasoning_delta":
			if event.Data.DeltaContent != nil {
				text := *event.Data.DeltaContent
				if cb.OnDelta != nil {
					cb.OnDelta(text)
				}
				_, _ = fmt.Fprint(stdout, text)
			}
		case "assistant.message":
			if event.Data.Content != nil {
				text := *event.Data.Content
				if cb.OnMessage != nil {
					cb.OnMessage(text)
				}
				_, _ = fmt.Fprintf(stdout, "assistant: %s\n", text)
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
		return fmt.Errorf("send copilot prompt: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	select {
	case <-done:
		return nil
	case err := <-errCh:
		return err
	case <-waitCtx.Done():
		return fmt.Errorf("wait copilot session idle: %w", waitCtx.Err())
	}
}

func defaultCopilotString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

// CopilotProvider implements Provider using Copilot SDK text completion.
type CopilotProvider struct {
	cfg CopilotConfig
}

// NewCopilot creates a Copilot-backed Provider.
func NewCopilot(cfg CopilotConfig) *CopilotProvider {
	return &CopilotProvider{cfg: cfg}
}

func (p *CopilotProvider) Name() string { return "copilot" }

func (p *CopilotProvider) Available() bool {
	if p == nil {
		return false
	}
	if !CopilotCLIExists(p.cfg.CLIPath) && !p.cfg.UseLoggedInUser {
		return false
	}
	return p.cfg.UseLoggedInUser || CopilotCLIExists(p.cfg.CLIPath)
}

func (p *CopilotProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	if !p.Available() {
		return CompleteResponse{}, fmt.Errorf("copilot provider unavailable")
	}

	prompt := composePrompt(req)
	var (
		mu      sync.Mutex
		lastMsg string
	)

	err := RunCopilotSession(ctx, p.cfg, prompt, SessionCallbacks{
		OnMessage: func(text string) {
			mu.Lock()
			lastMsg = strings.TrimSpace(text)
			mu.Unlock()
		},
	})
	if err != nil {
		return CompleteResponse{}, err
	}

	mu.Lock()
	text := lastMsg
	mu.Unlock()
	if text == "" {
		return CompleteResponse{}, fmt.Errorf("copilot completion: empty assistant message")
	}

	return CompleteResponse{
		Text:     text,
		Provider: p.Name(),
		Model:    defaultCopilotString(p.cfg.Model, "gpt-5"),
	}, nil
}

func composePrompt(req CompleteRequest) string {
	system := strings.TrimSpace(req.System)
	user := strings.TrimSpace(req.User)
	switch {
	case system != "" && user != "":
		return system + "\n\n" + user
	case user != "":
		return user
	default:
		return system
	}
}
