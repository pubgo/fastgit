package agentacp

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

type updateCollector struct {
	mu      sync.Mutex
	updates []acp.SessionNotification
}

func (c *updateCollector) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates = append(c.updates, params)
	return nil
}

func (c *updateCollector) snapshot() []acp.SessionNotification {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]acp.SessionNotification(nil), c.updates...)
}

func TestPromptTurnPermissionFlow(t *testing.T) {
	broker := NewPermissionBroker()
	client := &CallbackClient{PermissionBroker: broker}
	collector := &updateCollector{}

	var bridge *AgentBridge
	exec := PromptExecutorFunc(func(ctx context.Context, sessionID acp.SessionId, prompt []acp.ContentBlock, emit func(update acp.SessionUpdate) error) (acp.StopReason, error) {
		toolID := acp.ToolCallId("call_approve_1")
		title := "apply patch"
		if err := emit(acp.StartToolCall(toolID, title,
			acp.WithStartKind(acp.ToolKindEdit),
			acp.WithStartStatus(acp.ToolCallStatusPending),
		)); err != nil {
			return "", err
		}

		resp, err := bridge.RequestPermission(ctx, acp.RequestPermissionRequest{
			SessionId: sessionID,
			ToolCall: acp.RequestPermissionToolCall{
				ToolCallId: toolID,
				Title:      acp.Ptr(title),
				Kind:       acp.Ptr(acp.ToolKindEdit),
				Status:     acp.Ptr(acp.ToolCallStatusPending),
			},
			Options: []acp.PermissionOption{
				{OptionId: "allow-once", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce},
				{OptionId: "reject-once", Name: "Reject once", Kind: acp.PermissionOptionKindRejectOnce},
			},
		})
		if err != nil {
			return "", err
		}

		if resp.Outcome.Selected == nil || strings.TrimSpace(string(resp.Outcome.Selected.OptionId)) == "" {
			if err := emit(acp.UpdateToolCall(toolID,
				acp.WithUpdateStatus(acp.ToolCallStatusFailed),
				acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolContent(acp.TextBlock("permission denied"))}),
			)); err != nil {
				return "", err
			}
			return acp.StopReasonRefusal, nil
		}

		if err := emit(acp.UpdateToolCall(toolID,
			acp.WithUpdateStatus(acp.ToolCallStatusInProgress),
		)); err != nil {
			return "", err
		}
		if err := emit(acp.UpdateToolCall(toolID,
			acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
			acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolContent(acp.TextBlock("patch applied"))}),
		)); err != nil {
			return "", err
		}
		if err := emit(acp.UpdateAgentMessageText("done")); err != nil {
			return "", err
		}
		return acp.StopReasonEndTurn, nil
	})

	bridge = NewAgentBridge(BridgeOptions{Executor: exec, PermissionRequester: client})
	bridge.SetSessionUpdater(collector)

	newResp, err := bridge.NewSession(context.Background(), acp.NewSessionRequest{Cwd: "/tmp", McpServers: nil})
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}

	promptRespCh := make(chan acp.PromptResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, runErr := bridge.Prompt(context.Background(), acp.PromptRequest{
			SessionId: newResp.SessionId,
			Prompt:    []acp.ContentBlock{acp.TextBlock("please edit file")},
		})
		if runErr != nil {
			errCh <- runErr
			return
		}
		promptRespCh <- resp
	}()

	var pendingID string
	for i := 0; i < 100; i++ {
		pending := broker.Pending()
		if len(pending) > 0 {
			pendingID = pending[0].RequestID
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.TrimSpace(pendingID) == "" {
		t.Fatal("permission request not observed")
	}

	if err := broker.ResolveFirstByKind(pendingID, acp.PermissionOptionKindAllowOnce); err != nil {
		t.Fatalf("resolve permission failed: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("prompt returned error: %v", err)
	case resp := <-promptRespCh:
		if resp.StopReason != acp.StopReasonEndTurn {
			t.Fatalf("unexpected stop reason: %s", resp.StopReason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting prompt response")
	}

	updates := collector.snapshot()
	if len(updates) == 0 {
		t.Fatal("expected session updates")
	}

	joined := make([]string, 0, len(updates))
	for _, u := range updates {
		if u.Update.ToolCall != nil {
			joined = append(joined, "tool_call:"+string(u.Update.ToolCall.Status))
		}
		if u.Update.ToolCallUpdate != nil && u.Update.ToolCallUpdate.Status != nil {
			joined = append(joined, "tool_update:"+string(*u.Update.ToolCallUpdate.Status))
		}
		if u.Update.AgentMessageChunk != nil && u.Update.AgentMessageChunk.Content.Text != nil {
			joined = append(joined, "assistant:"+u.Update.AgentMessageChunk.Content.Text.Text)
		}
	}
	out := strings.Join(joined, " | ")
	if !strings.Contains(out, "tool_call:pending") {
		t.Fatalf("expected pending tool call status, got: %s", out)
	}
	if !strings.Contains(out, "tool_update:in_progress") || !strings.Contains(out, "tool_update:completed") {
		t.Fatalf("expected in_progress/completed status, got: %s", out)
	}
	if !strings.Contains(out, "assistant:done") {
		t.Fatalf("expected final assistant message, got: %s", out)
	}
}
