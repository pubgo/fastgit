package acpclient

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

const fakeAgentMarker = "FASTGIT_ACP_FAKE_AGENT"

// TestFakeAgentProcess doubles as the entrypoint when the test binary is
// re-spawned as a fake ACP agent over stdio (see newFakeAgent).
func TestFakeAgentProcess(t *testing.T) {
	if os.Getenv(fakeAgentMarker) != "1" {
		t.Skip("only runs when spawned as a fake acp agent")
	}
	agent := &fakeAgent{mode: os.Getenv("FAKE_MODE"), reply: os.Getenv("FAKE_REPLY")}
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn
	<-conn.Done()
	os.Exit(0)
}

func newFakeAgent(mode, reply string) Agent {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return Agent{
		Name:    "fake",
		Command: exe,
		Args:    []string{"-test.run=TestFakeAgentProcess", "-test.timeout=120s"},
		Env: map[string]string{
			fakeAgentMarker: "1",
			"FAKE_MODE":     mode,
			"FAKE_REPLY":    reply,
		},
	}
}

type fakeAgent struct {
	mode  string
	reply string
	conn  *acp.AgentSideConnection
}

func (a *fakeAgent) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (a *fakeAgent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "fake-session"}, nil
}

func (a *fakeAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	switch a.mode {
	case "deny":
		_, err := a.conn.RequestPermission(ctx, acp.RequestPermissionRequest{
			SessionId: params.SessionId,
			ToolCall:  acp.ToolCallUpdate{ToolCallId: "t1", Title: acp.Ptr("run git push")},
			Options: []acp.PermissionOption{
				{OptionId: "allow-once", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce},
			},
		})
		if err != nil {
			return acp.PromptResponse{}, err
		}
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	case "empty":
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	case "hang":
		select {
		case <-ctx.Done():
			return acp.PromptResponse{}, ctx.Err()
		case <-time.After(30 * time.Second):
			return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
		}
	case "fs-write":
		if _, err := a.conn.WriteTextFile(ctx, acp.WriteTextFileRequest{
			SessionId: params.SessionId, Path: "out.txt", Content: "hello fs",
		}); err != nil {
			return acp.PromptResponse{}, err
		}
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: params.SessionId,
			Update:    acp.UpdateAgentMessageText("wrote out.txt"),
		}); err != nil {
			return acp.PromptResponse{}, err
		}
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	case "fs-escape":
		_, err := a.conn.WriteTextFile(ctx, acp.WriteTextFileRequest{
			SessionId: params.SessionId, Path: "../escape.txt", Content: "nope",
		})
		text := "sandbox-error"
		if err == nil {
			text = "sandbox-bypassed"
		}
		if e := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: params.SessionId,
			Update:    acp.UpdateAgentMessageText(text),
		}); e != nil {
			return acp.PromptResponse{}, e
		}
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	case "ask-allow":
		resp, err := a.conn.RequestPermission(ctx, acp.RequestPermissionRequest{
			SessionId: params.SessionId,
			ToolCall:  acp.ToolCallUpdate{ToolCallId: "t1", Title: acp.Ptr("edit Unreleased.md")},
			Options: []acp.PermissionOption{
				{OptionId: "al", Name: "Always allow", Kind: acp.PermissionOptionKindAllowAlways},
				{OptionId: "on", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce},
			},
		})
		if err != nil {
			return acp.PromptResponse{}, err
		}
		text := "cancelled"
		if resp.Outcome.Selected != nil {
			text = "selected:" + string(resp.Outcome.Selected.OptionId)
		}
		if e := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: params.SessionId,
			Update:    acp.UpdateAgentMessageText(text),
		}); e != nil {
			return acp.PromptResponse{}, e
		}
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	default: // reply
		text := a.reply
		if text == "" {
			text = "fix(cli): add acp\nprovider"
		}
		i := strings.LastIndex(text, "\n")
		if i < 0 {
			i = len(text) / 2
		}
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: params.SessionId,
			Update:    acp.UpdateAgentMessageText(text[:i]),
		}); err != nil {
			return acp.PromptResponse{}, err
		}
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: params.SessionId,
			Update:    acp.UpdateAgentMessageText(text[i:]),
		}); err != nil {
			return acp.PromptResponse{}, err
		}
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	}
}

func (a *fakeAgent) Authenticate(ctx context.Context, params acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, acp.NewMethodNotFound("authenticate")
}

func (a *fakeAgent) Logout(ctx context.Context, params acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, acp.NewMethodNotFound("logout")
}

func (a *fakeAgent) Cancel(ctx context.Context, params acp.CancelNotification) error {
	return nil
}

func (a *fakeAgent) CloseSession(ctx context.Context, params acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}

func (a *fakeAgent) ListSessions(ctx context.Context, params acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, acp.NewMethodNotFound("session/list")
}

func (a *fakeAgent) ResumeSession(ctx context.Context, params acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, acp.NewMethodNotFound("session/resume")
}

func (a *fakeAgent) SetSessionConfigOption(ctx context.Context, params acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound("session/set_config_option")
}

func (a *fakeAgent) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, acp.NewMethodNotFound("session/set_mode")
}

func TestRunCollectsAgentMessageChunks(t *testing.T) {
	agent := newFakeAgent("reply", "")
	result, err := Run(context.Background(), agent, Options{Prompt: "write a commit message", Timeout: 60 * time.Second})
	require.NoError(t, err)
	require.Equal(t, "fix(cli): add acp\nprovider", result.Text)
	require.Equal(t, acp.StopReasonEndTurn, result.StopReason)
}

func TestRunPermissionDenied(t *testing.T) {
	agent := newFakeAgent("deny", "")
	_, err := Run(context.Background(), agent, Options{Prompt: "hi", Timeout: 60 * time.Second})
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission")
}

func TestRunEmptyText(t *testing.T) {
	agent := newFakeAgent("empty", "")
	_, err := Run(context.Background(), agent, Options{Prompt: "hi", Timeout: 60 * time.Second})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no agent_message text")
}

func TestRunTimeout(t *testing.T) {
	agent := newFakeAgent("hang", "")
	_, err := Run(context.Background(), agent, Options{Prompt: "hi", Timeout: 800 * time.Millisecond})
	require.Error(t, err)
}

func TestRunStartError(t *testing.T) {
	_, err := Run(context.Background(), Agent{Name: "missing", Command: "definitely-not-a-real-binary-xyz"}, Options{Prompt: "hi"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "start")
}

func TestRunEmptyCommand(t *testing.T) {
	_, err := Run(context.Background(), Agent{Name: "empty"}, Options{Prompt: "hi"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "command is empty")
}

func TestRunClientFSWriteSandboxed(t *testing.T) {
	dir := t.TempDir()
	agent := newFakeAgent("fs-write", "")
	result, err := Run(context.Background(), agent, Options{Prompt: "write", Cwd: dir, ClientFS: true, Timeout: 60 * time.Second})
	require.NoError(t, err)
	require.Equal(t, "wrote out.txt", result.Text)

	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello fs", string(data))
}

func TestRunClientFSEscapeDenied(t *testing.T) {
	dir := t.TempDir()
	agent := newFakeAgent("fs-escape", "")
	result, err := Run(context.Background(), agent, Options{Prompt: "escape", Cwd: dir, ClientFS: true, Timeout: 60 * time.Second})
	require.NoError(t, err)
	require.Equal(t, "sandbox-error", result.Text)
	require.NoFileExists(t, filepath.Join(filepath.Dir(dir), "escape.txt"))
}

func TestRunClientFSNotAdvertised(t *testing.T) {
	dir := t.TempDir()
	agent := newFakeAgent("fs-write", "")
	_, err := Run(context.Background(), agent, Options{Prompt: "write", Cwd: dir, Timeout: 60 * time.Second})
	require.Error(t, err) // agent got method-not-found and failed the prompt
}

func TestRunPermissionAllowOnce(t *testing.T) {
	agent := newFakeAgent("ask-allow", "")
	result, err := Run(context.Background(), agent, Options{
		Prompt:       "edit it",
		Timeout:      60 * time.Second,
		OnPermission: PermissionAllowOnce,
	})
	require.NoError(t, err)
	require.Equal(t, "selected:on", result.Text) // allow_once preferred over allow_always
}

func TestRunPermissionInteractiveDeny(t *testing.T) {
	agent := newFakeAgent("ask-allow", "")
	result, err := Run(context.Background(), agent, Options{
		Prompt:  "edit it",
		Timeout: 60 * time.Second,
		OnPermission: PermissionInteractive(func(title string, options []acp.PermissionOption) string {
			require.Equal(t, "edit Unreleased.md", title)
			return "" // reject
		}),
	})
	require.NoError(t, err)
	require.Equal(t, "cancelled", result.Text)
}

func TestPermissionAllowOnceNoAllowOption(t *testing.T) {
	_, err := PermissionAllowOnce(context.Background(), acp.RequestPermissionRequest{
		Options: []acp.PermissionOption{{OptionId: "no", Name: "No", Kind: acp.PermissionOptionKindRejectOnce}},
	})
	require.NoError(t, err)
}

func TestBuildPrompt(t *testing.T) {
	require.Equal(t, "user", buildPrompt(Options{Prompt: "user"}))
	require.Equal(t, "sys", buildPrompt(Options{System: "sys"}))
	require.Equal(t, "sys\n\nuser", buildPrompt(Options{System: "sys", Prompt: "user"}))
}

func TestMergeEnv(t *testing.T) {
	out := mergeEnv([]string{"A=1", "B=2"}, map[string]string{"B": "override", "C": "3"})
	require.Contains(t, out, "A=1")
	require.Contains(t, out, "B=override")
	require.Contains(t, out, "C=3")
	require.NotContains(t, out, "B=2")
}
