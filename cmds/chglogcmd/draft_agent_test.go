package chglogcmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/pubgo/fastgit/pkg/acpconfig"
	"github.com/pubgo/redant"
	"github.com/stretchr/testify/require"
)

const draftFakeAgentMarker = "FASTGIT_DRAFT_FAKE_AGENT"

// TestDraftFakeAgentProcess doubles as a fake ACP agent that writes the
// changelog file through client fs and replies with a summary.
func TestDraftFakeAgentProcess(t *testing.T) {
	if os.Getenv(draftFakeAgentMarker) != "1" {
		t.Skip("only runs when spawned as a fake acp agent")
	}
	agent := &draftFakeAgent{target: os.Getenv("FAKE_TARGET_PATH")}
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn
	<-conn.Done()
	os.Exit(0)
}

type draftFakeAgent struct {
	target string
	conn   *acp.AgentSideConnection
}

const draftFakeContent = `# [Unreleased]

## 新增

- fake agent 写回的条目

## 修复

- 无

## 变更

- 无

## 文档

- 无

## 影响范围

- 测试仓库

## 验证建议

- 跑测试

## 回滚建议

- 回退提交
`

func (a *draftFakeAgent) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (a *draftFakeAgent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "draft-fake"}, nil
}

func (a *draftFakeAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	if _, err := a.conn.WriteTextFile(ctx, acp.WriteTextFileRequest{
		SessionId: params.SessionId, Path: a.target, Content: draftFakeContent,
	}); err != nil {
		return acp.PromptResponse{}, err
	}
	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: params.SessionId,
		Update:    acp.UpdateAgentMessageText("updated unreleased"),
	}); err != nil {
		return acp.PromptResponse{}, err
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (a *draftFakeAgent) Authenticate(ctx context.Context, params acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, acp.NewMethodNotFound("authenticate")
}

func (a *draftFakeAgent) Logout(ctx context.Context, params acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, acp.NewMethodNotFound("logout")
}

func (a *draftFakeAgent) Cancel(ctx context.Context, params acp.CancelNotification) error { return nil }

func (a *draftFakeAgent) CloseSession(ctx context.Context, params acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}

func (a *draftFakeAgent) ListSessions(ctx context.Context, params acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, acp.NewMethodNotFound("session/list")
}

func (a *draftFakeAgent) ResumeSession(ctx context.Context, params acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, acp.NewMethodNotFound("session/resume")
}

func (a *draftFakeAgent) SetSessionConfigOption(ctx context.Context, params acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound("session/set_config_option")
}

func (a *draftFakeAgent) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, acp.NewMethodNotFound("session/set_mode")
}

func setupDraftRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "--initial-branch=main")
	runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	if _, err := ensureChangelogScaffold(repo, scaffoldOptions{Version: "v0.0.1", CreateVersionIfMissing: true}); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "scaffold changelog")
	return repo
}

func overrideACPConfig(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	old := acpconfig.ConfigPathFn
	acpconfig.ConfigPathFn = func() string { return path }
	t.Cleanup(func() { acpconfig.ConfigPathFn = old })
}

func TestRunDraftWithAgentHappyPath(t *testing.T) {
	repo := setupDraftRepo(t)
	paths := buildPaths(repo)
	exe, err := os.Executable()
	require.NoError(t, err)

	overrideACPConfig(t, "acp:\n  default: draftfake\n  agents:\n    draftfake:\n      command: "+exe+"\n      args: [\"-test.run=TestDraftFakeAgentProcess\", \"-test.timeout=120s\"]\n      env:\n        "+draftFakeAgentMarker+": \"1\"\n        FAKE_TARGET_PATH: \""+paths.UnreleasedFile+"\"\n")

	buf := &bytes.Buffer{}
	inv := &redant.Invocation{Stdout: buf, Stderr: buf}
	err = runDraftWithAgent(context.Background(), inv, "update the changelog", repo, agentParams{
		Permit:   "deny", // fake agent never requests permission
		Timeout:  "60",
		ClientFS: true,
	})
	require.NoError(t, err)

	out := buf.String()
	require.Contains(t, out, "draft updated")
	require.NotContains(t, out, "note:")

	data, err := os.ReadFile(paths.UnreleasedFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "fake agent 写回的条目")
}

func TestRunDraftWithAgentNoAgentConfigured(t *testing.T) {
	repo := setupDraftRepo(t)
	overrideACPConfig(t, "version:\n  name: v0.0.1\n")
	t.Setenv(acpconfig.EnvAgentVar, "ghost-agent")

	buf := &bytes.Buffer{}
	inv := &redant.Invocation{Stdout: buf, Stderr: buf}
	err := runDraftWithAgent(context.Background(), inv, "update", repo, agentParams{Timeout: "30"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "ghost-agent"))
}

func TestRunDraftWithAgentUnchangedFileWarns(t *testing.T) {
	repo := setupDraftRepo(t)
	overrideACPConfig(t, "acp:\n  agents:\n    broken:\n      command: sh\n      args: [\"-c\", \"exit 7\"]\n")

	buf := &bytes.Buffer{}
	inv := &redant.Invocation{Stdout: buf, Stderr: buf}
	err := runDraftWithAgent(context.Background(), inv, "update", repo, agentParams{Timeout: "10"})
	require.Error(t, err) // fails at initialize: not an acp agent
	require.Contains(t, err.Error(), "broken")
}
