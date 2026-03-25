package agentlineapp

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

func displayStart(start, end int) int {
	if end == 0 {
		return 0
	}
	return start + 1
}

func visibleSuggestionRange(total, selected, maxRows int) (start, end int) {
	if total <= 0 {
		return 0, 0
	}
	if maxRows <= 0 {
		maxRows = 1
	}
	if total <= maxRows {
		return 0, total
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	start = selected - maxRows/2
	if start < 0 {
		start = 0
	}
	if start+maxRows > total {
		start = total - maxRows
	}
	end = start + maxRows
	return start, end
}

func visibleOutputRange(total, rows, offset int) (start, end int) {
	if total <= 0 {
		return 0, 0
	}
	if rows <= 0 {
		rows = 1
	}
	if total <= rows {
		return 0, total
	}

	offset = clampOutputOffset(offset, total, rows)
	end = total - offset
	start = end - rows
	return start, end
}

func visibleInputRange(total, rows, offset int) (start, end int) {
	if total <= 0 {
		return 0, 0
	}
	if rows <= 0 {
		rows = 1
	}
	if total <= rows {
		return 0, total
	}

	offset = clampInputOffset(offset, total, rows)
	end = total - offset
	start = end - rows
	return start, end
}

func clampOutputOffset(offset, total, rows int) int {
	if offset < 0 {
		offset = 0
	}
	if total <= 0 {
		return 0
	}
	if rows <= 0 {
		rows = 1
	}
	maxOffset := total - rows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func clampInputOffset(offset, total, rows int) int {
	if offset < 0 {
		offset = 0
	}
	if total <= 0 {
		return 0
	}
	if rows <= 0 {
		rows = 1
	}
	maxOffset := total - rows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func uniqueCompletionItems(items []completionItem) []completionItem {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]completionItem, 0, len(items))
	for _, item := range items {
		ins := strings.TrimSpace(item.Insert)
		if ins == "" {
			continue
		}
		if _, ok := seen[ins]; ok {
			continue
		}
		seen[ins] = struct{}{}
		item.Insert = ins
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Insert != out[j].Insert {
			return out[i].Insert < out[j].Insert
		}
		return out[i].Description < out[j].Description
	})
	return out
}

func padRightDisplay(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = truncateDisplayWidth(s, width)
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func truncateDisplayWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	ellipsis := "…"
	ellipsisWidth := lipgloss.Width(ellipsis)
	if maxWidth <= ellipsisWidth {
		return ellipsis
	}

	target := maxWidth - ellipsisWidth
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > target {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + ellipsis
}

func wrapDisplayWidth(s string, maxWidth int) []string {
	s = strings.ReplaceAll(s, "\t", "    ")
	if maxWidth <= 0 || lipgloss.Width(s) <= maxWidth {
		return []string{s}
	}

	var lines []string
	var cur strings.Builder
	curWidth := 0

	flush := func() {
		lines = append(lines, cur.String())
		cur.Reset()
		curWidth = 0
	}

	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if rw <= 0 {
			rw = 1
		}
		if curWidth > 0 && curWidth+rw > maxWidth {
			flush()
		}
		cur.WriteRune(r)
		curWidth += rw
	}
	if cur.Len() > 0 {
		flush()
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func normalizeOutputLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.Contains(part, "\r") {
			seg := strings.Split(part, "\r")
			part = seg[len(seg)-1]
		}
		part = strings.TrimRight(part, "\r")
		out = append(out, part)
	}
	return out
}

func renderBlockHeader(kind blockKind, text string) string {
	switch kind {
	case blockKindSystem:
		return styleKindSystem.Render(text)
	case blockKindUser:
		return styleKindUser.Render(text)
	case blockKindAssistant:
		return styleKindAssistant.Render(text)
	case blockKindTool:
		return styleKindTool.Render(text)
	case blockKindCommand:
		return styleKindCommand.Render(text)
	case blockKindResult:
		return styleKindResult.Render(text)
	case blockKindError:
		return styleKindError.Render(text)
	default:
		return text
	}
}
