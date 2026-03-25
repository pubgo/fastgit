package agentlineapp

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	acp "github.com/coder/acp-go-sdk"
)

func TestHandleSlashInput_PermissionsList(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	respCh := make(chan acp.RequestPermissionResponse, 1)
	go func() {
		resp, _ := m.permissionBroker.RequestPermission(context.Background(), acp.RequestPermissionRequest{
			SessionId: "sess_1",
			ToolCall:  acp.RequestPermissionToolCall{ToolCallId: "call_1", Title: acp.Ptr("edit file")},
			Options:   []acp.PermissionOption{{OptionId: "allow-once", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce}},
		})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(m.permissionBroker.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	handled, cmd := m.handleSlashInput("/permissions")
	if !handled || cmd != nil {
		t.Fatalf("expected /permissions handled without async cmd")
	}
	last := m.blocks[len(m.blocks)-1]
	if last.Title != "/permissions" {
		t.Fatalf("expected /permissions block title, got %q", last.Title)
	}
	if !strings.Contains(strings.Join(last.Lines, "\n"), "request=perm_") {
		t.Fatalf("expected pending permission lines")
	}

	// 清理请求，避免 goroutine 泄漏。
	_ = m.permissionBroker.ResolveCancelled(m.permissionBroker.Pending()[0].RequestID)
	select {
	case <-respCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting permission response")
	}
}

func TestHandleSlashInput_AllowResolvesRequest(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	respCh := make(chan acp.RequestPermissionResponse, 1)
	go func() {
		resp, _ := m.permissionBroker.RequestPermission(context.Background(), acp.RequestPermissionRequest{
			SessionId: "sess_1",
			ToolCall:  acp.RequestPermissionToolCall{ToolCallId: "call_1"},
			Options:   []acp.PermissionOption{{OptionId: "allow-once", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce}},
		})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(m.permissionBroker.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	handled, cmd := m.handleSlashInput("/allow")
	if !handled || cmd != nil {
		t.Fatalf("expected /allow handled without async cmd")
	}

	select {
	case resp := <-respCh:
		if resp.Outcome.Selected == nil {
			t.Fatalf("expected selected outcome")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting allow response")
	}
}

func TestHandleSlashInput_DenyResolvesRequest(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	respCh := make(chan acp.RequestPermissionResponse, 1)
	go func() {
		resp, _ := m.permissionBroker.RequestPermission(context.Background(), acp.RequestPermissionRequest{
			SessionId: "sess_1",
			ToolCall:  acp.RequestPermissionToolCall{ToolCallId: "call_1"},
			Options:   []acp.PermissionOption{{OptionId: "reject-once", Name: "Reject once", Kind: acp.PermissionOptionKindRejectOnce}},
		})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(m.permissionBroker.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	handled, cmd := m.handleSlashInput("/deny")
	if !handled || cmd != nil {
		t.Fatalf("expected /deny handled without async cmd")
	}

	select {
	case resp := <-respCh:
		if resp.Outcome.Selected == nil {
			t.Fatalf("expected selected reject outcome")
		}
		if resp.Outcome.Selected.OptionId != "reject-once" {
			t.Fatalf("expected reject-once, got %s", resp.Outcome.Selected.OptionId)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting deny response")
	}
}

func TestIsAllowedWhileRunning(t *testing.T) {
	if !isAllowedWhileRunning("/allow") {
		t.Fatalf("expected /allow allowed while running")
	}
	if !isAllowedWhileRunning("/permissions") {
		t.Fatalf("expected /permissions allowed while running")
	}
	if isAllowedWhileRunning("/run commit") {
		t.Fatalf("expected /run not allowed while running")
	}
	if isAllowedWhileRunning("plain text") {
		t.Fatalf("expected plain text not allowed while running")
	}
}

func TestEnterWhileRunningBlocksNonPermissionSlash(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.running = true
	m.input.SetValue("/run commit --message hi")

	model, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(*agentlineModel)
	if cmd != nil {
		t.Fatalf("expected no async cmd when blocked by running guard")
	}
	last := m.blocks[len(m.blocks)-1]
	if last.Title != "running" {
		t.Fatalf("expected running hint block, got %q", last.Title)
	}
}

func TestHandleSlashInput_ACPDemoStartsRunning(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/acp-demo test prompt")
	if !handled {
		t.Fatalf("expected /acp-demo handled")
	}
	if cmd == nil {
		t.Fatalf("expected /acp-demo to return async cmd")
	}
	if !m.running {
		t.Fatalf("expected running=true after /acp-demo")
	}
}
