package agentlineapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

func TestHandleSlashInput_ACPEventsTimeline(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	m.appendACPSessionNotification(acp.SessionNotification{
		SessionId: "sess_1",
		Update:    acp.UpdateUserMessage(acp.TextBlock("hello")),
	})
	m.appendACPSessionNotification(acp.SessionNotification{
		SessionId: "sess_1",
		Update:    acp.UpdateAgentMessageText("hi"),
	})

	handled, cmd := m.handleSlashInput("/acp-events 10")
	if !handled || cmd != nil {
		t.Fatalf("expected /acp-events handled without async cmd")
	}

	last := m.blocks[len(m.blocks)-1]
	if last.Title != "/acp-events" {
		t.Fatalf("expected /acp-events block title, got %q", last.Title)
	}
	joined := strings.Join(last.Lines, "\n")
	if !strings.Contains(joined, "total:") {
		t.Fatalf("expected timeline total line, got: %s", joined)
	}
	if !strings.Contains(joined, "assistant_message") {
		t.Fatalf("expected assistant_message in timeline, got: %s", joined)
	}
}

func TestHandleSlashInput_ACPEventsExport(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	m.appendACPSessionNotification(acp.SessionNotification{
		SessionId: "sess_export",
		Update:    acp.UpdateAgentMessageText("export me"),
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "data.jsonl")
	handled, cmd := m.handleSlashInput("/acp-events-export " + path)
	if !handled || cmd != nil {
		t.Fatalf("expected /acp-events-export handled without async cmd")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported file failed: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, "sess_export") {
		t.Fatalf("expected exported session id in jsonl: %s", content)
	}
	if !strings.Contains(content, "assistant_message") {
		t.Fatalf("expected exported kind in jsonl: %s", content)
	}
}
