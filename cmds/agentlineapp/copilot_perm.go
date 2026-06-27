package agentlineapp

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pubgo/fastgit/pkg/copilotperm"
)

func (m *agentlineModel) initCopilotPermissionBroker() {
	if m == nil {
		return
	}
	if m.copilotPermBroker == nil {
		m.copilotPermBroker = copilotperm.NewBroker()
	}
	copilotperm.SetGlobalBroker(m.copilotPermBroker)
}

func (m *agentlineModel) copilotPermissionLines() []string {
	if m == nil || m.copilotPermBroker == nil {
		return nil
	}
	pending := m.copilotPermBroker.Pending()
	if len(pending) == 0 {
		return nil
	}
	lines := make([]string, 0, len(pending)*2)
	for i, item := range pending {
		lines = append(lines, fmt.Sprintf("copilot %d) request=%s kind=%s tool=%s", i+1, item.RequestID, item.Kind, item.ToolName))
		if strings.TrimSpace(item.Summary) != "" {
			lines = append(lines, "   summary: "+strings.TrimSpace(item.Summary))
		}
	}
	return lines
}

func (m *agentlineModel) resolveCopilotPermissionSlash(allow bool, argText string) tea.Cmd {
	if m == nil || m.copilotPermBroker == nil {
		return nil
	}
	pending := m.copilotPermBroker.Pending()
	if len(pending) == 0 {
		return nil
	}

	title := "/deny"
	if allow {
		title = "/allow"
	}

	requestID, err := copilotperm.ParseCopilotRequestArg(pending, strings.TrimSpace(argText))
	if err != nil {
		m.appendBlock(sessionBlock{Kind: blockKindError, Title: title, Lines: []string{err.Error()}})
		return nil
	}
	if err := m.copilotPermBroker.Resolve(requestID, allow); err != nil {
		m.appendBlock(sessionBlock{Kind: blockKindError, Title: title, Lines: []string{err.Error()}})
		return nil
	}
	m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: title, Lines: []string{fmt.Sprintf("已处理 copilot request=%s", requestID)}})
	return nil
}
