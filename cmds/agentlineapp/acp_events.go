package agentlineapp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

const maxACPEventEntries = 2000

type acpEventEntry struct {
	Seq          int64
	CapturedAt   time.Time
	Notification acp.SessionNotification
}

type acpEventExportRecord struct {
	Seq        int64                   `json:"seq"`
	CapturedAt string                  `json:"captured_at"`
	SessionID  string                  `json:"session_id"`
	Kind       string                  `json:"kind"`
	Summary    string                  `json:"summary"`
	Raw        acp.SessionNotification `json:"raw"`
}

func (m *agentlineModel) recordACPEvent(params acp.SessionNotification) {
	if m == nil {
		return
	}
	m.acpEventSeq++
	m.acpEventEntries = append(m.acpEventEntries, acpEventEntry{
		Seq:          m.acpEventSeq,
		CapturedAt:   time.Now(),
		Notification: params,
	})
	if len(m.acpEventEntries) > maxACPEventEntries {
		extra := len(m.acpEventEntries) - maxACPEventEntries
		m.acpEventEntries = append([]acpEventEntry(nil), m.acpEventEntries[extra:]...)
	}
}

func (m *agentlineModel) acpEventsSnapshot() []acpEventEntry {
	if m == nil || len(m.acpEventEntries) == 0 {
		return nil
	}
	return append([]acpEventEntry(nil), m.acpEventEntries...)
}

func (m *agentlineModel) acpEventsTimelineLines(limit int) []string {
	events := m.acpEventsSnapshot()
	if len(events) == 0 {
		return []string{"暂无 ACP 事件。先运行 /acp-demo 触发一轮交互。"}
	}

	if limit <= 0 {
		limit = 40
	}
	start := 0
	if len(events) > limit {
		start = len(events) - limit
	}

	lines := make([]string, 0, len(events)-start+2)
	lines = append(lines, fmt.Sprintf("total: %d, showing: %d-%d", len(events), start+1, len(events)))
	for i := start; i < len(events); i++ {
		e := events[i]
		kind := acpEventKind(e.Notification)
		summary := compactEventText(acpEventSummary(e.Notification), 120)
		lines = append(lines, fmt.Sprintf("%03d [%s] t=%s session=%s %s", e.Seq, kind, e.CapturedAt.Format("15:04:05.000"), strings.TrimSpace(string(e.Notification.SessionId)), summary))
	}
	return lines
}

func (m *agentlineModel) acpEventsSummaryLines() []string {
	events := m.acpEventsSnapshot()
	if len(events) == 0 {
		return []string{"暂无 ACP 事件。"}
	}

	kindCount := map[string]int{}
	sessionCount := map[string]int{}
	for _, e := range events {
		kindCount[acpEventKind(e.Notification)]++
		session := strings.TrimSpace(string(e.Notification.SessionId))
		if session == "" {
			session = "(empty)"
		}
		sessionCount[session]++
	}

	lines := []string{fmt.Sprintf("total events: %d", len(events))}
	lines = append(lines, "kind counts:")
	for _, item := range sortedCountItems(kindCount) {
		lines = append(lines, fmt.Sprintf("  - %s: %d", item.key, item.value))
	}
	lines = append(lines, "session counts:")
	for _, item := range sortedCountItems(sessionCount) {
		lines = append(lines, fmt.Sprintf("  - %s: %d", item.key, item.value))
	}
	return lines
}

type kvCount struct {
	key   string
	value int
}

func sortedCountItems(m map[string]int) []kvCount {
	items := make([]kvCount, 0, len(m))
	for k, v := range m {
		items = append(items, kvCount{key: k, value: v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].value != items[j].value {
			return items[i].value > items[j].value
		}
		return items[i].key < items[j].key
	})
	return items
}

func (m *agentlineModel) exportACPEventsJSONL(path string) (int, error) {
	events := m.acpEventsSnapshot()
	if len(events) == 0 {
		return 0, nil
	}

	path = strings.TrimSpace(path)
	if path == "" {
		path = ".local/data.jsonl"
	}
	dir := filepath.Dir(path)
	if strings.TrimSpace(dir) != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	written := 0
	for _, e := range events {
		rec := acpEventExportRecord{
			Seq:        e.Seq,
			CapturedAt: e.CapturedAt.Format(time.RFC3339Nano),
			SessionID:  strings.TrimSpace(string(e.Notification.SessionId)),
			Kind:       acpEventKind(e.Notification),
			Summary:    acpEventSummary(e.Notification),
			Raw:        e.Notification,
		}
		if err := enc.Encode(rec); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

func parsePositiveIntOrDefault(raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func acpEventKind(n acp.SessionNotification) string {
	u := n.Update
	switch {
	case u.UserMessageChunk != nil:
		return "user_message"
	case u.AgentMessageChunk != nil:
		return "assistant_message"
	case u.AgentThoughtChunk != nil:
		return "thought"
	case u.ToolCall != nil:
		return "tool_call"
	case u.ToolCallUpdate != nil:
		return "tool_update"
	case u.Plan != nil:
		return "plan"
	case u.AvailableCommandsUpdate != nil:
		return "commands_update"
	case u.CurrentModeUpdate != nil:
		return "mode_update"
	default:
		return "unknown"
	}
}

func acpEventSummary(n acp.SessionNotification) string {
	blocks := sessionBlocksFromACP(n)
	if len(blocks) == 0 {
		return "(no rendered blocks)"
	}
	first := blocks[0]
	line := ""
	if len(first.Lines) > 0 {
		line = strings.TrimSpace(first.Lines[0])
	}
	if line == "" {
		line = strings.TrimSpace(first.Title)
	}
	if line == "" {
		line = "(empty)"
	}
	return line
}

func compactEventText(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.Join(strings.Fields(s), " ")
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}
