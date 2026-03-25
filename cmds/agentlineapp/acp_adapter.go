package agentlineapp

import (
	"strings"

	acp "github.com/coder/acp-go-sdk"

	agentacp "github.com/pubgo/redant/cmds/agentlineapp/acp"
)

// sessionBlocksFromACP 将 ACP session/update 转换为 agentline 输出块。
func sessionBlocksFromACP(params acp.SessionNotification) []sessionBlock {
	rendered := agentacp.RenderSessionNotification(params)
	if len(rendered) == 0 {
		return nil
	}

	out := make([]sessionBlock, 0, len(rendered))
	for _, item := range rendered {
		kind := mapACPBlockKind(item.Kind)
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = string(kind)
		}
		out = append(out, sessionBlock{
			Kind:  kind,
			Title: title,
			Lines: append([]string(nil), item.Lines...),
		})
	}

	return out
}

// appendACPSessionNotification 将 ACP 事件直接写入当前会话输出。
func (m *agentlineModel) appendACPSessionNotification(params acp.SessionNotification) {
	if m == nil {
		return
	}
	m.recordACPEvent(params)
	blocks := sessionBlocksFromACP(params)
	if len(blocks) == 0 {
		return
	}
	m.appendBlocks(blocks)
	m.outputOffset = 0
	m.normalizeOutputOffset()
}

func mapACPBlockKind(kind string) blockKind {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "user":
		return blockKindUser
	case "assistant":
		return blockKindAssistant
	case "tool":
		return blockKindTool
	case "result":
		return blockKindResult
	case "error":
		return blockKindError
	default:
		return blockKindSystem
	}
}
