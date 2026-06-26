package agentacp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

type captureUpdater struct {
	mu      sync.Mutex
	updates []acp.SessionNotification
}

func (c *captureUpdater) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates = append(c.updates, params)
	return nil
}

func (c *captureUpdater) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.updates)
}

func TestAgentBridgeInitializeAndPrompt(t *testing.T) {
	bridge := NewAgentBridge(BridgeOptions{AgentInfo: acp.Implementation{Name: "redant", Version: "test"}})
	updater := &captureUpdater{}
	bridge.SetSessionUpdater(updater)

	initResp, err := bridge.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersion(acp.ProtocolVersionNumber)})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	if initResp.ProtocolVersion != acp.ProtocolVersion(acp.ProtocolVersionNumber) {
		t.Fatalf("unexpected protocol version: %v", initResp.ProtocolVersion)
	}

	newResp, err := bridge.NewSession(context.Background(), acp.NewSessionRequest{Cwd: "/tmp", McpServers: nil})
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}

	promptResp, err := bridge.Prompt(context.Background(), acp.PromptRequest{
		SessionId: newResp.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock("hello")},
	})
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}
	if promptResp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("unexpected stop reason: %s", promptResp.StopReason)
	}

	if updater.count() < 2 {
		t.Fatalf("expected at least 2 updates(user+assistant), got %d", updater.count())
	}
}

func TestAgentBridgeCancelStopsPrompt(t *testing.T) {
	start := make(chan struct{})
	exec := PromptExecutorFunc(func(ctx context.Context, sessionID acp.SessionId, prompt []acp.ContentBlock, emit func(update acp.SessionUpdate) error) (acp.StopReason, error) {
		close(start)
		<-ctx.Done()
		return acp.StopReasonCancelled, ctx.Err()
	})

	bridge := NewAgentBridge(BridgeOptions{Executor: exec})
	newResp, err := bridge.NewSession(context.Background(), acp.NewSessionRequest{Cwd: "/tmp", McpServers: nil})
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}

	respCh := make(chan acp.PromptResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, runErr := bridge.Prompt(context.Background(), acp.PromptRequest{SessionId: newResp.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("long task")}})
		if runErr != nil {
			errCh <- runErr
			return
		}
		respCh <- resp
	}()

	select {
	case <-start:
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not start")
	}

	if err := bridge.Cancel(context.Background(), acp.CancelNotification{SessionId: newResp.SessionId}); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected prompt error: %v", err)
		}
	case resp := <-respCh:
		if resp.StopReason != acp.StopReasonCancelled {
			t.Fatalf("expected cancelled stop reason, got %s", resp.StopReason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("prompt did not finish after cancel")
	}
}

func TestCallbackClientDefaultPermissionCancelled(t *testing.T) {
	client := &CallbackClient{}
	resp, err := client.RequestPermission(context.Background(), acp.RequestPermissionRequest{})
	if err != nil {
		t.Fatalf("request permission failed: %v", err)
	}
	if resp.Outcome.Cancelled == nil {
		t.Fatalf("expected default cancelled outcome")
	}
}
