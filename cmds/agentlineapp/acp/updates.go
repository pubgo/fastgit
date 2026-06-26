package agentacp

import (
	"fmt"
	"strings"

	acp "github.com/coder/acp-go-sdk"
)

// RenderedBlock 是从 ACP 事件提炼出的可展示结构。
type RenderedBlock struct {
	Kind  string
	Title string
	Lines []string
}

// RenderSessionNotification 将 ACP session/update 转换为可展示块。
func RenderSessionNotification(params acp.SessionNotification) []RenderedBlock {
	update := params.Update
	blocks := make([]RenderedBlock, 0, 2)

	if u := update.UserMessageChunk; u != nil {
		text := strings.TrimSpace(contentBlockSummary(u.Content))
		if text == "" {
			text = "(empty user content)"
		}
		blocks = append(blocks, RenderedBlock{Kind: "user", Title: "user", Lines: []string{text}})
	}

	if u := update.AgentMessageChunk; u != nil {
		text := strings.TrimSpace(contentBlockSummary(u.Content))
		if text == "" {
			text = "(empty assistant content)"
		}
		blocks = append(blocks, RenderedBlock{Kind: "assistant", Title: "assistant", Lines: []string{text}})
	}

	if u := update.AgentThoughtChunk; u != nil {
		text := strings.TrimSpace(contentBlockSummary(u.Content))
		if text == "" {
			text = "(empty thought content)"
		}
		blocks = append(blocks, RenderedBlock{Kind: "system", Title: "thought", Lines: []string{text}})
	}

	if u := update.ToolCall; u != nil {
		lines := []string{
			fmt.Sprintf("id: %s", strings.TrimSpace(string(u.ToolCallId))),
			fmt.Sprintf("status: %s", strings.TrimSpace(string(u.Status))),
			fmt.Sprintf("kind: %s", strings.TrimSpace(string(u.Kind))),
		}
		contentLines := toolContentSummaries(u.Content)
		if len(contentLines) > 0 {
			lines = append(lines, contentLines...)
		}
		blocks = append(blocks, RenderedBlock{Kind: "tool", Title: withDefault(strings.TrimSpace(u.Title), "tool_call"), Lines: lines})
	}

	if u := update.ToolCallUpdate; u != nil {
		lines := []string{fmt.Sprintf("id: %s", strings.TrimSpace(string(u.ToolCallId)))}
		if u.Status != nil {
			lines = append(lines, fmt.Sprintf("status: %s", strings.TrimSpace(string(*u.Status))))
		}
		if u.Kind != nil {
			lines = append(lines, fmt.Sprintf("kind: %s", strings.TrimSpace(string(*u.Kind))))
		}
		contentLines := toolContentSummaries(u.Content)
		if len(contentLines) > 0 {
			lines = append(lines, contentLines...)
		}
		title := "tool_update"
		if u.Title != nil && strings.TrimSpace(*u.Title) != "" {
			title = strings.TrimSpace(*u.Title)
		}
		blocks = append(blocks, RenderedBlock{Kind: "tool", Title: title, Lines: lines})
	}

	if u := update.Plan; u != nil {
		lines := make([]string, 0, len(u.Entries))
		for idx, entry := range u.Entries {
			lines = append(lines, fmt.Sprintf("%d. [%s/%s] %s", idx+1, strings.TrimSpace(string(entry.Status)), strings.TrimSpace(string(entry.Priority)), strings.TrimSpace(entry.Content)))
		}
		if len(lines) == 0 {
			lines = append(lines, "(empty plan)")
		}
		blocks = append(blocks, RenderedBlock{Kind: "system", Title: "plan", Lines: lines})
	}

	if u := update.AvailableCommandsUpdate; u != nil {
		lines := make([]string, 0, len(u.AvailableCommands))
		for _, c := range u.AvailableCommands {
			name := strings.TrimSpace(c.Name)
			desc := strings.TrimSpace(c.Description)
			if name == "" {
				continue
			}
			if desc == "" {
				lines = append(lines, name)
			} else {
				lines = append(lines, fmt.Sprintf("%s: %s", name, desc))
			}
		}
		if len(lines) == 0 {
			lines = append(lines, "(no available commands)")
		}
		blocks = append(blocks, RenderedBlock{Kind: "system", Title: "commands", Lines: lines})
	}

	if u := update.CurrentModeUpdate; u != nil {
		blocks = append(blocks, RenderedBlock{Kind: "system", Title: "mode", Lines: []string{fmt.Sprintf("current mode: %s", strings.TrimSpace(string(u.CurrentModeId)))}})
	}

	return blocks
}

func contentBlockSummary(block acp.ContentBlock) string {
	if block.Text != nil {
		return strings.TrimSpace(block.Text.Text)
	}
	if block.ResourceLink != nil {
		name := strings.TrimSpace(block.ResourceLink.Name)
		uri := strings.TrimSpace(block.ResourceLink.Uri)
		if name == "" {
			name = "resource"
		}
		if uri == "" {
			return name
		}
		return fmt.Sprintf("%s (%s)", name, uri)
	}
	if block.Resource != nil {
		if block.Resource.Resource.TextResourceContents != nil {
			textRes := block.Resource.Resource.TextResourceContents
			return fmt.Sprintf("resource text: %s", strings.TrimSpace(textRes.Uri))
		}
		if block.Resource.Resource.BlobResourceContents != nil {
			blobRes := block.Resource.Resource.BlobResourceContents
			return fmt.Sprintf("resource blob: %s", strings.TrimSpace(blobRes.Uri))
		}
		return "resource"
	}
	if block.Image != nil {
		return fmt.Sprintf("image: %s", strings.TrimSpace(block.Image.MimeType))
	}
	if block.Audio != nil {
		return fmt.Sprintf("audio: %s", strings.TrimSpace(block.Audio.MimeType))
	}
	return ""
}

func toolContentSummaries(contents []acp.ToolCallContent) []string {
	lines := make([]string, 0, len(contents))
	for _, c := range contents {
		if c.Content != nil {
			s := strings.TrimSpace(contentBlockSummary(c.Content.Content))
			if s != "" {
				lines = append(lines, s)
			}
			continue
		}
		if c.Diff != nil {
			lines = append(lines, fmt.Sprintf("diff: %s", strings.TrimSpace(c.Diff.Path)))
			continue
		}
		if c.Terminal != nil {
			lines = append(lines, fmt.Sprintf("terminal: %s", strings.TrimSpace(c.Terminal.TerminalId)))
		}
	}
	return lines
}

func withDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
