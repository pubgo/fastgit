package agentlineapp

import (
	"context"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

func TestSessionBlocksFromACP_AgentAndToolUpdates(t *testing.T) {
	notification := acp.SessionNotification{
		SessionId: "sess_1",
		Update: acp.UpdateToolCall("call_1",
			acp.WithUpdateTitle("run tests"),
			acp.WithUpdateKind(acp.ToolKindExecute),
			acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
			acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolContent(acp.TextBlock("ok"))}),
		),
	}

	blocks := sessionBlocksFromACP(notification)
	if len(blocks) != 1 {
		t.Fatalf("expected one block, got %d", len(blocks))
	}
	if blocks[0].Kind != blockKindTool {
		t.Fatalf("expected tool block kind, got %s", blocks[0].Kind)
	}
	joined := strings.Join(blocks[0].Lines, "\n")
	if !strings.Contains(joined, "status: completed") {
		t.Fatalf("expected completed status, got: %s", joined)
	}
	if !strings.Contains(joined, "ok") {
		t.Fatalf("expected tool content summary, got: %s", joined)
	}
}

func TestAppendACPSessionNotification_AppendsBlocks(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	initial := len(m.blocks)

	m.appendACPSessionNotification(acp.SessionNotification{
		SessionId: "sess_1",
		Update:    acp.UpdateAgentMessageText("hello from acp"),
	})

	if len(m.blocks) != initial+1 {
		t.Fatalf("expected block count +1, got initial=%d current=%d", initial, len(m.blocks))
	}
	last := m.blocks[len(m.blocks)-1]
	if last.Kind != blockKindAssistant {
		t.Fatalf("expected assistant kind, got %s", last.Kind)
	}
	if !strings.Contains(strings.Join(last.Lines, "\n"), "hello from acp") {
		t.Fatalf("expected ACP message in output block")
	}
}

func TestSessionBlocksFromACP_PlanAndMode(t *testing.T) {
	planUpdate := acp.UpdatePlan(acp.PlanEntry{
		Content:  "执行回归测试",
		Priority: acp.PlanEntryPriorityHigh,
		Status:   acp.PlanEntryStatusInProgress,
	})
	modeUpdate := acp.SessionUpdate{
		CurrentModeUpdate: &acp.SessionCurrentModeUpdate{
			SessionUpdate: "current_mode_update",
			CurrentModeId: "code",
		},
	}

	planBlocks := sessionBlocksFromACP(acp.SessionNotification{SessionId: "sess_1", Update: planUpdate})
	if len(planBlocks) != 1 {
		t.Fatalf("expected one plan block, got %d", len(planBlocks))
	}
	if planBlocks[0].Kind != blockKindSystem {
		t.Fatalf("expected system block for plan, got %s", planBlocks[0].Kind)
	}
	if !strings.Contains(strings.Join(planBlocks[0].Lines, "\n"), "执行回归测试") {
		t.Fatalf("expected plan content in plan block")
	}

	modeBlocks := sessionBlocksFromACP(acp.SessionNotification{SessionId: "sess_1", Update: modeUpdate})
	if len(modeBlocks) != 1 {
		t.Fatalf("expected one mode block, got %d", len(modeBlocks))
	}
	if !strings.Contains(strings.Join(modeBlocks[0].Lines, "\n"), "current mode: code") {
		t.Fatalf("expected mode summary")
	}
}
