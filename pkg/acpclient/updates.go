package acpclient

import (
	"fmt"

	acp "github.com/coder/acp-go-sdk"
)

// UpdateLine renders a session/update into a short one-line progress string.
// Returns empty string for updates not worth showing.
func UpdateLine(update acp.SessionUpdate) string {
	switch {
	case update.ToolCall != nil:
		status := string(update.ToolCall.Status)
		if status == "" {
			status = "pending"
		}
		return fmt.Sprintf("🔧 %s [%s]", update.ToolCall.Title, status)
	case update.ToolCallUpdate != nil:
		title := ""
		if update.ToolCallUpdate.Title != nil {
			title = " " + *update.ToolCallUpdate.Title
		}
		status := ""
		if update.ToolCallUpdate.Status != nil {
			status = string(*update.ToolCallUpdate.Status)
		}
		return fmt.Sprintf("🔧 %s%s [%s]", update.ToolCallUpdate.ToolCallId, title, status)
	case update.Plan != nil:
		return fmt.Sprintf("📋 plan: %d entries", len(update.Plan.Entries))
	case update.AgentThoughtChunk != nil:
		return ""
	default:
		return ""
	}
}
