package agentlineapp

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *agentlineModel) View() tea.View {
	contentWidth := m.contentWidth()

	renderedOutput := m.renderOutputLines(contentWidth)
	outputRows := m.outputRows()
	outputOffset := clampOutputOffset(m.outputOffset, len(renderedOutput), outputRows)
	outputStart, outputEnd := visibleOutputRange(len(renderedOutput), outputRows, outputOffset)

	statusText := "IDLE"
	statusStyle := styleStatusIdle
	if m.running {
		statusText = "RUNNING"
		statusStyle = styleStatusBusy
	}
	pendingQuestions := 0
	latestQuestion := ""
	if m.questionBroker != nil {
		pending := m.questionBroker.Pending()
		pendingQuestions = len(pending)
		if pendingQuestions > 0 {
			latestQuestion = strings.TrimSpace(pending[pendingQuestions-1].Prompt)
			if m.running {
				statusText = "WAITING_ANSWER"
				statusStyle = styleStatusBusy
			}
		}
	}
	status := statusStyle.Render(statusText)
	mode := strings.ToUpper(string(m.mode))
	if strings.TrimSpace(mode) == "" {
		mode = strings.ToUpper(string(interactionModeCommand))
	}
	focus := "INPUT"
	if m.outputFocus {
		focus = "OUTPUT"
	}

	lines := make([]string, 0, m.height+8)
	header := fmt.Sprintf("agentline · status=%s · mode=%s · focus=%s · blocks=%d · lines=%d", status, mode, focus, len(m.blocks), len(renderedOutput))
	if pendingQuestions > 0 {
		header += fmt.Sprintf(" · pending_questions=%d", pendingQuestions)
	}
	lines = append(lines, styleHeader.Render(truncateDisplayWidth(header, contentWidth)))
	lines = append(lines, styleHint.Render(truncateDisplayWidth(m.sessionContextLine(), contentWidth)))
	if binding := strings.TrimSpace(m.chatBindingLine()); binding != "" {
		lines = append(lines, styleHint.Render(truncateDisplayWidth(binding, contentWidth)))
	}

	outputTitle := fmt.Sprintf("输出区域（%d-%d/%d）", displayStart(outputStart, outputEnd), outputEnd, len(renderedOutput))
	lines = append(lines, styleHeader.Render(truncateDisplayWidth(outputTitle, contentWidth)))
	outputRegionStart := len(lines) - 1
	if len(renderedOutput) == 0 {
		lines = append(lines, styleHint.Render("暂无输出"))
	} else {
		for i := outputStart; i < outputEnd; i++ {
			lines = append(lines, renderedOutput[i])
		}
	}

	if len(m.suggestions) > 0 {
		rows := m.suggestionRows(len(m.suggestions))
		s, e := visibleSuggestionRange(len(m.suggestions), m.selected, rows)
		suggestionHeader := fmt.Sprintf("slash 候选（%d，显示 %d-%d）", len(m.suggestions), s+1, e)
		lines = append(lines, styleHeader.Render(truncateDisplayWidth(suggestionHeader, contentWidth)))

		suggestionWidth := contentWidth
		if suggestionWidth > 0 {
			suggestionWidth -= 2
		}

		for i := s; i < e; i++ {
			item := m.suggestions[i]
			prefix := "  "
			if i == m.selected {
				prefix = "> "
			}
			line := padRightDisplay(item.Insert, 18)
			raw := line
			if item.Description != "" {
				raw += " " + styleDesc.Render(item.Description)
			}
			raw = truncateDisplayWidth(raw, suggestionWidth)
			if i == m.selected {
				raw = styleSelected.Render(raw)
			}
			lines = append(lines, prefix+raw)
		}
		lines = append(lines, "  "+styleHint.Render("提示：↑/↓ 选择，Tab 应用，Esc 关闭候选"))
	}

	if m.running {
		if pendingQuestions > 0 {
			lines = append(lines, styleRunning.Render(truncateDisplayWidth(fmt.Sprintf("执行中（等待问题回复，pending=%d）", pendingQuestions), contentWidth)))
			if latestQuestion != "" {
				lines = append(lines, styleHint.Render(truncateDisplayWidth("最新问题: "+latestQuestion, contentWidth)))
			}
			lines = append(lines, styleHint.Render(truncateDisplayWidth("可直接输入并回车回复，或使用 /questions、/reply、/skip。", contentWidth)))
		} else {
			lines = append(lines, styleRunning.Render(truncateDisplayWidth("执行中...", contentWidth)))
		}
	}
	outputRegionEnd := len(lines) - 1

	inputTitle := "输入区域（支持点击历史回填）"
	inputRegionStart := len(lines)
	lines = append(lines, styleHeader.Render(truncateDisplayWidth(inputTitle, contentWidth)))
	lines = append(lines, m.input.View())

	historyRendered, historyIndices := m.renderInputHistoryLinesWithIndices(contentWidth)
	historyRows := m.inputRows()
	historyStart, historyEnd := visibleInputRange(len(historyRendered), historyRows, m.inputOffset)

	historyTitle := fmt.Sprintf("最近历史（%d-%d/%d，可点击回填）", displayStart(historyStart, historyEnd), historyEnd, len(historyRendered))
	if len(historyRendered) == 0 {
		historyTitle = "最近历史（暂无）"
	}
	lines = append(lines, styleHeader.Render(truncateDisplayWidth(historyTitle, contentWidth)))

	historyRegionStart := len(lines)
	if len(historyRendered) == 0 {
		lines = append(lines, styleHint.Render("暂无输入历史"))
	} else {
		for i := historyStart; i < historyEnd; i++ {
			line := historyRendered[i]
			if i >= 0 && i < len(historyIndices) && historyIndices[i] == m.selectedHistory {
				line = styleHistorySelected.Render(line)
			}
			lines = append(lines, line)
		}
	}
	historyRegionEnd := len(lines) - 1

	lines = append(lines, styleHint.Render(truncateDisplayWidth("命令：/run /history /cancel /fold /unfold；支持直接鼠标拖拽选择复制。", contentWidth)))
	if m.isChatMode() {
		lines = append(lines, styleHint.Render(truncateDisplayWidth("当前为聊天模式：普通输入会作为已绑定命令的 prompt；可继续使用 slash 命令（如 /run、/history），/unbind 可退出。", contentWidth)))
	}
	inputRegionEnd := len(lines) - 1

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.OnMouse = func(msg tea.MouseMsg) tea.Cmd {
		switch event := msg.(type) {
		case tea.MouseWheelMsg:
			mouse := event.Mouse()
			if mouse.Mod&tea.ModShift != 0 {
				// Shift+鼠标事件旁路给终端原生处理，便于文本选择与复制。
				return nil
			}
			delta := 0
			switch mouse.Button {
			case tea.MouseWheelUp:
				delta = 1
			case tea.MouseWheelDown:
				delta = -1
			default:
				return nil
			}

			if mouse.Y < outputRegionStart || mouse.Y > inputRegionEnd {
				return nil
			}

			region := mouseRegionOutput
			if mouse.Y >= inputRegionStart && mouse.Y <= inputRegionEnd {
				region = mouseRegionInput
			} else if mouse.Y > outputRegionEnd {
				return nil
			}

			return func() tea.Msg {
				return mouseScrollMsg{Region: region, Delta: delta}
			}

		case tea.MouseClickMsg:
			mouse := event.Mouse()
			if mouse.Mod&tea.ModShift != 0 {
				// Shift+点击旁路给终端，允许原生选择。
				return nil
			}
			y := mouse.Y

			if y < outputRegionStart || y > inputRegionEnd {
				return nil
			}

			if y >= outputRegionStart && y <= outputRegionEnd {
				return func() tea.Msg {
					return mouseFocusMsg{Region: mouseRegionOutput}
				}
			}

			if y >= inputRegionStart && y <= inputRegionEnd {
				if y >= historyRegionStart && y <= historyRegionEnd && len(historyRendered) > 0 {
					row := y - historyRegionStart
					renderedIndex := historyStart + row
					if renderedIndex >= 0 && renderedIndex < len(historyIndices) {
						hIdx := historyIndices[renderedIndex]
						return func() tea.Msg {
							return mouseSelectHistoryMsg{HistoryIndex: hIdx}
						}
					}
				}

				return func() tea.Msg {
					return mouseFocusMsg{Region: mouseRegionInput}
				}
			}

			return nil
		}

		return nil
	}

	return v
}

func (m *agentlineModel) scrollOutputLines(delta int) {
	total := len(m.renderOutputLines(m.contentWidth()))
	rows := m.outputRows()
	m.outputOffset = clampOutputOffset(m.outputOffset+delta, total, rows)
}

func (m *agentlineModel) scrollInputLines(delta int) {
	total := len(m.renderInputHistoryLines(m.contentWidth()))
	rows := m.inputRows()
	m.inputOffset = clampInputOffset(m.inputOffset+delta, total, rows)
}

func (m *agentlineModel) scrollOutputPage(deltaPage int) {
	if deltaPage == 0 {
		return
	}
	rows := m.outputRows()
	m.scrollOutputLines(deltaPage * rows)
}

func (m *agentlineModel) scrollOutputTop() {
	total := len(m.renderOutputLines(m.contentWidth()))
	rows := m.outputRows()
	maxOffset := total - rows
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.outputOffset = maxOffset
}

func (m *agentlineModel) scrollOutputBottom() {
	m.outputOffset = 0
}

func (m *agentlineModel) normalizeOutputOffset() {
	total := len(m.renderOutputLines(m.contentWidth()))
	m.outputOffset = clampOutputOffset(m.outputOffset, total, m.outputRows())
}

func (m *agentlineModel) normalizeInputOffset() {
	total := len(m.renderInputHistoryLines(m.contentWidth()))
	m.inputOffset = clampInputOffset(m.inputOffset, total, m.inputRows())
}

func (m *agentlineModel) renderOutputLines(width int) []string {
	if len(m.blocks) == 0 {
		return nil
	}

	out := make([]string, 0, len(m.blocks)*3)
	for i, block := range m.blocks {
		title := strings.TrimSpace(block.Title)
		if title == "" {
			title = string(block.Kind)
		}

		head := fmt.Sprintf("■ #%d [%s] %s", i+1, strings.ToUpper(string(block.Kind)), title)
		out = append(out, renderBlockHeader(block.Kind, truncateDisplayWidth(head, width)))

		linesToRender := block.Lines
		if m.foldDetails && (block.Kind == blockKindAssistant || block.Kind == blockKindTool) && len(block.Lines) > 1 {
			linesToRender = []string{
				block.Lines[0],
				fmt.Sprintf("... (%d more lines folded)", len(block.Lines)-1),
			}
		}

		if len(linesToRender) == 0 {
			out = append(out, "  (no output)")
		} else {
			for _, line := range linesToRender {
				wrapped := wrapDisplayWidth(line, width-2)
				if len(wrapped) == 0 {
					continue
				}
				for _, w := range wrapped {
					out = append(out, "  "+w)
				}
			}
		}

		if i < len(m.blocks)-1 {
			sep := "────────────────"
			if width > 0 {
				sep = strings.Repeat("─", width)
			}
			out = append(out, sep)
		}
	}

	return out
}

func (m *agentlineModel) renderInputHistoryLines(width int) []string {
	lines, _ := m.renderInputHistoryLinesWithIndices(width)
	return lines
}

func (m *agentlineModel) renderInputHistoryLinesWithIndices(width int) ([]string, []int) {
	if len(m.history) == 0 {
		return nil, nil
	}

	historyWidth := width
	if historyWidth > 0 {
		historyWidth -= 2
	}
	out := make([]string, 0, len(m.history))
	indices := make([]int, 0, len(m.history))
	for i, line := range m.history {
		entry := fmt.Sprintf("%03d %s", i+1, strings.TrimSpace(line))
		wrapped := wrapDisplayWidth(entry, historyWidth)
		if len(wrapped) == 0 {
			continue
		}
		for _, w := range wrapped {
			out = append(out, w)
			indices = append(indices, i)
		}
	}
	return out, indices
}

func (m *agentlineModel) contentWidth() int {
	if m.width <= 0 {
		return 0
	}
	w := m.width - 1
	if w < 1 {
		return 1
	}
	return w
}

func (m *agentlineModel) suggestionRows(total int) int {
	if total <= 0 {
		return 1
	}

	rows := defaultSuggestionRows
	if m.height <= 0 {
		if total < rows {
			return total
		}
		return rows
	}

	available := m.height - m.baseOccupiedRows(true) - minOutputRows
	if available < 1 {
		available = 1
	}
	if available > rows {
		available = rows
	}
	if available > total {
		available = total
	}
	return available
}

func (m *agentlineModel) inputRows() int {
	if m.height <= 0 {
		return defaultInputRows
	}

	rows := defaultInputRows
	maxRows := m.height / 3
	if maxRows < 1 {
		maxRows = 1
	}
	if rows > maxRows {
		rows = maxRows
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *agentlineModel) outputRows() int {
	if m.height <= 0 {
		return defaultOutputRows
	}

	occupied := m.baseOccupiedRows(false)
	if len(m.suggestions) > 0 {
		occupied += 3 + m.suggestionRows(len(m.suggestions))
	}

	rows := m.height - occupied
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *agentlineModel) baseOccupiedRows(withSuggestionFrame bool) int {
	rows := 5
	if m.running {
		rows++
	}
	if withSuggestionFrame {
		rows += 2
	}
	return rows
}
