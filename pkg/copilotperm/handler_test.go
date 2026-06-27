package copilotperm

import (
	"bytes"
	"context"
	"strings"
	"testing"

	copilot "github.com/github/copilot-sdk/go"
	"github.com/stretchr/testify/require"
)

type memoryAuditor struct {
	entries []AuditEntry
}

func (m *memoryAuditor) Log(entry AuditEntry) {
	m.entries = append(m.entries, entry)
}

func TestParseMode(t *testing.T) {
	mode, err := ParseMode("allow")
	require.NoError(t, err)
	require.Equal(t, ModeAllow, mode)

	_, err = ParseMode("bogus")
	require.Error(t, err)
}

func TestHandlerAllowAndDeny(t *testing.T) {
	tool := "write_file"
	req := copilot.PermissionRequest{Kind: copilot.PermissionRequestKindWrite, ToolName: &tool}
	audit := &memoryAuditor{}

	allow := NewHandler(Options{Mode: ModeAllow, Auditor: audit})
	result, err := allow(req, copilot.PermissionInvocation{SessionID: "s1"})
	require.NoError(t, err)
	require.Equal(t, copilot.PermissionRequestResultKindApproved, result.Kind)

	deny := NewHandler(Options{Mode: ModeDeny, Auditor: audit})
	result, err = deny(req, copilot.PermissionInvocation{SessionID: "s1"})
	require.NoError(t, err)
	require.Equal(t, copilot.PermissionRequestResultKindDeniedInteractivelyByUser, result.Kind)
	require.Len(t, audit.entries, 2)
}

func TestHandlerAskUsesPrompter(t *testing.T) {
	tool := "bash"
	req := copilot.PermissionRequest{Kind: copilot.PermissionRequestKindShell, ToolName: &tool}
	audit := &memoryAuditor{}

	yesHandler := NewHandler(Options{
		Mode:    ModeAsk,
		Auditor: audit,
		Prompter: func(_ context.Context, _ copilot.PermissionRequest, _ string) (bool, error) {
			return true, nil
		},
	})
	result, err := yesHandler(req, copilot.PermissionInvocation{})
	require.NoError(t, err)
	require.Equal(t, copilot.PermissionRequestResultKindApproved, result.Kind)

	noHandler := NewHandler(Options{
		Mode: ModeAsk,
		Prompter: func(_ context.Context, _ copilot.PermissionRequest, _ string) (bool, error) {
			return false, nil
		},
	})
	result, err = noHandler(req, copilot.PermissionInvocation{})
	require.NoError(t, err)
	require.Equal(t, copilot.PermissionRequestResultKindDeniedInteractivelyByUser, result.Kind)
}

func TestDefaultTerminalPrompter(t *testing.T) {
	in := strings.NewReader("yes\n")
	var out bytes.Buffer
	prompter := DefaultTerminalPrompter(in, &out)
	ok, err := prompter(context.Background(), copilot.PermissionRequest{Kind: copilot.PermissionRequestKindRead}, "read file")
	require.NoError(t, err)
	require.True(t, ok)
	require.Contains(t, out.String(), "Allow?")
}
