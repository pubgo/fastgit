package agentacp

import (
	"context"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

func TestPermissionBroker_RequestResolveSelected(t *testing.T) {
	b := NewPermissionBroker()
	respCh := make(chan acp.RequestPermissionResponse, 1)

	go func() {
		resp, _ := b.RequestPermission(context.Background(), acp.RequestPermissionRequest{
			SessionId: "sess_1",
			ToolCall:  acp.RequestPermissionToolCall{ToolCallId: "call_1"},
			Options:   []acp.PermissionOption{{OptionId: "allow-once", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce}},
		})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(b.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	pending := b.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}

	if err := b.ResolveFirstByKind(pending[0].RequestID, acp.PermissionOptionKindAllowOnce, acp.PermissionOptionKindAllowAlways); err != nil {
		t.Fatalf("resolve by kind failed: %v", err)
	}

	select {
	case resp := <-respCh:
		if resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != "allow-once" {
			t.Fatalf("unexpected selected outcome: %+v", resp.Outcome)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting response")
	}
}

func TestPermissionBroker_RequestCancelOnContextDone(t *testing.T) {
	b := NewPermissionBroker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := b.RequestPermission(ctx, acp.RequestPermissionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Outcome.Cancelled == nil {
		t.Fatalf("expected cancelled outcome")
	}
}

func TestCallbackClient_UsesPermissionBroker(t *testing.T) {
	b := NewPermissionBroker()
	client := &CallbackClient{PermissionBroker: b}

	respCh := make(chan acp.RequestPermissionResponse, 1)
	go func() {
		resp, _ := client.RequestPermission(context.Background(), acp.RequestPermissionRequest{
			SessionId: "sess_1",
			ToolCall:  acp.RequestPermissionToolCall{ToolCallId: "call_1"},
			Options:   []acp.PermissionOption{{OptionId: "reject-once", Name: "Reject once", Kind: acp.PermissionOptionKindRejectOnce}},
		})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(b.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	pending := b.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}

	if err := b.ResolveByIndex(pending[0].RequestID, 1); err != nil {
		t.Fatalf("resolve by index failed: %v", err)
	}

	select {
	case resp := <-respCh:
		if resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != "reject-once" {
			t.Fatalf("unexpected selected outcome: %+v", resp.Outcome)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting broker response")
	}
}
