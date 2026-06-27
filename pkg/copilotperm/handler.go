package copilotperm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	copilot "github.com/github/copilot-sdk/go"
)

// Prompter decides whether to approve an interactive permission request.
type Prompter func(ctx context.Context, request copilot.PermissionRequest, summary string) (bool, error)

// Options configures a permission handler.
type Options struct {
	Mode      Mode
	SessionID string
	Auditor   Auditor
	Prompter  Prompter
}

// NewHandler returns a Copilot OnPermissionRequest handler for the given mode.
func NewHandler(opts Options) copilot.PermissionHandlerFunc {
	mode := opts.Mode
	if mode == "" {
		mode = DefaultMode
	}

	return func(request copilot.PermissionRequest, invocation copilot.PermissionInvocation) (copilot.PermissionRequestResult, error) {
		summary := summarizeRequest(request)
		sessionID := strings.TrimSpace(opts.SessionID)
		if sessionID == "" {
			sessionID = strings.TrimSpace(invocation.SessionID)
		}

		logDecision := func(decision string) {
			if opts.Auditor == nil {
				return
			}
			opts.Auditor.Log(AuditEntry{
				SessionID: sessionID,
				Kind:      string(request.Kind),
				ToolName:  deref(request.ToolName),
				Decision:  decision,
				Mode:      mode,
				Summary:   summary,
			})
		}

		switch mode {
		case ModeAllow:
			logDecision("approved")
			return copilot.PermissionRequestResult{Kind: copilot.PermissionRequestResultKindApproved}, nil
		case ModeDeny:
			logDecision("denied")
			return copilot.PermissionRequestResult{Kind: copilot.PermissionRequestResultKindDeniedInteractivelyByUser}, nil
		default:
			prompter := opts.Prompter
			if prompter == nil {
				prompter = DefaultTerminalPrompter(os.Stdin, os.Stdout)
			}
			ok, err := prompter(context.Background(), request, summary)
			if err != nil {
				logDecision("error")
				return copilot.PermissionRequestResult{}, err
			}
			if ok {
				logDecision("approved")
				return copilot.PermissionRequestResult{Kind: copilot.PermissionRequestResultKindApproved}, nil
			}
			logDecision("denied")
			return copilot.PermissionRequestResult{Kind: copilot.PermissionRequestResultKindDeniedInteractivelyByUser}, nil
		}
	}
}

// DefaultTerminalPrompter asks y/N on stdin.
func DefaultTerminalPrompter(in io.Reader, out io.Writer) Prompter {
	return func(_ context.Context, _ copilot.PermissionRequest, summary string) (bool, error) {
		_, _ = fmt.Fprintf(out, "Copilot permission request:\n  %s\nAllow? [y/N]: ", summary)
		reader := bufio.NewReader(in)
		line, err := reader.ReadString('\n')
		if err != nil && !strings.Contains(err.Error(), "EOF") {
			return false, err
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		return answer == "y" || answer == "yes", nil
	}
}

func summarizeRequest(request copilot.PermissionRequest) string {
	parts := []string{string(request.Kind)}
	if request.ToolName != nil && strings.TrimSpace(*request.ToolName) != "" {
		parts = append(parts, "tool="+strings.TrimSpace(*request.ToolName))
	}
	if request.FileName != nil && strings.TrimSpace(*request.FileName) != "" {
		parts = append(parts, "file="+strings.TrimSpace(*request.FileName))
	}
	if request.FullCommandText != nil && strings.TrimSpace(*request.FullCommandText) != "" {
		parts = append(parts, "cmd="+strings.TrimSpace(*request.FullCommandText))
	}
	if request.Intention != nil && strings.TrimSpace(*request.Intention) != "" {
		parts = append(parts, strings.TrimSpace(*request.Intention))
	}
	return strings.Join(parts, " | ")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
