package ggccmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/yarlson/tap"
)

type uiMode string

const (
	modeSearch   uiMode = "search"
	modeWorkflow uiMode = "workflow"
)

type uiAction string

const (
	actionNone            uiAction = "none"
	actionExecuteSelected uiAction = "execute_selected"
	actionExecuteWorkflow uiAction = "execute_workflow"
)

type interactiveResult struct {
	Action      uiAction
	SelectedKey string
	SelectedUse string
	Workflow    []string
}

type interactiveModel struct {
	entries []CommandEntry

	mode  uiMode
	query string

	filtered []CommandEntry
	cursor   int

	workflows [][]string
	activeWF  int
	dirty     bool

	result interactiveResult
}

func newInteractiveModel(entries []CommandEntry, workflows [][]string) *interactiveModel {
	if len(workflows) == 0 {
		workflows = [][]string{{}}
	}

	copied := make([][]string, len(workflows))
	for i := range workflows {
		copied[i] = append([]string{}, workflows[i]...)
	}

	m := &interactiveModel{
		entries:   entries,
		mode:      modeSearch,
		workflows: copied,
		activeWF:  0,
		filtered:  entries,
		cursor:    0,
		dirty:     false,
		result:    interactiveResult{Action: actionNone},
	}
	m.applyFilter()
	return m
}

func (m *interactiveModel) Init() tea.Cmd { return nil }

func (m *interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.result = interactiveResult{Action: actionNone}
			return m, tea.Quit
		case tea.KeyCtrlT:
			if m.mode == modeSearch {
				m.mode = modeWorkflow
			} else {
				m.mode = modeSearch
			}
			return m, nil
		}

		if m.mode == modeSearch {
			return m.updateSearch(msg)
		}
		return m.updateWorkflow(msg)
	}

	return m, nil
}

func (m *interactiveModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlP:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown, tea.KeyCtrlN:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.query) > 0 {
			runes := []rune(m.query)
			m.query = string(runes[:len(runes)-1])
			m.applyFilter()
		}
	case tea.KeyTab:
		if len(m.filtered) > 0 {
			m.workflows[m.activeWF] = append(m.workflows[m.activeWF], m.filtered[m.cursor].Key)
			m.dirty = true
		}
	case tea.KeyEnter:
		if len(m.filtered) > 0 {
			m.result = interactiveResult{Action: actionExecuteSelected, SelectedKey: m.filtered[m.cursor].Key, SelectedUse: m.filtered[m.cursor].Usage}
			return m, tea.Quit
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.query += msg.String()
			m.applyFilter()
		}
	}

	return m, nil
}

func (m *interactiveModel) updateWorkflow(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlN, tea.KeyDown:
		if len(m.workflows) > 0 {
			m.activeWF = (m.activeWF + 1) % len(m.workflows)
		}
	case tea.KeyCtrlP, tea.KeyUp:
		if len(m.workflows) > 0 {
			m.activeWF = (m.activeWF - 1 + len(m.workflows)) % len(m.workflows)
		}
	case tea.KeyCtrlD:
		m.deleteActiveWorkflow()
	case tea.KeyEnter:
		if len(m.workflows[m.activeWF]) > 0 {
			wf := make([]string, len(m.workflows[m.activeWF]))
			copy(wf, m.workflows[m.activeWF])
			m.result = interactiveResult{Action: actionExecuteWorkflow, Workflow: wf}
			return m, tea.Quit
		}
	default:
		s := strings.TrimSpace(msg.String())
		switch s {
		case "n":
			m.workflows = append(m.workflows, []string{})
			m.activeWF = len(m.workflows) - 1
			m.dirty = true
		case "d":
			m.deleteActiveWorkflow()
		case "c":
			m.workflows[m.activeWF] = []string{}
			m.dirty = true
		case "x":
			if len(m.workflows[m.activeWF]) > 0 {
				wf := make([]string, len(m.workflows[m.activeWF]))
				copy(wf, m.workflows[m.activeWF])
				m.result = interactiveResult{Action: actionExecuteWorkflow, Workflow: wf}
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *interactiveModel) deleteActiveWorkflow() {
	if len(m.workflows) <= 1 {
		m.workflows[0] = []string{}
		m.activeWF = 0
		m.dirty = true
		return
	}

	m.workflows = append(m.workflows[:m.activeWF], m.workflows[m.activeWF+1:]...)
	if m.activeWF >= len(m.workflows) {
		m.activeWF = len(m.workflows) - 1
	}
	m.dirty = true
}

func (m *interactiveModel) applyFilter() {
	if strings.TrimSpace(m.query) == "" {
		m.filtered = m.entries
	} else {
		query := strings.TrimSpace(m.query)
		filtered := make([]CommandEntry, 0, len(m.entries))
		for _, entry := range m.entries {
			if fuzzy.MatchFold(query, entry.Usage) || fuzzy.MatchFold(query, entry.Description) || fuzzy.MatchFold(query, entry.Key) {
				filtered = append(filtered, entry)
			}
		}
		m.filtered = filtered
	}

	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}

	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
}

func (m *interactiveModel) View() string {
	var b strings.Builder
	b.WriteString("ggc interactive mode\n")
	b.WriteString("Search keys: type to filter, ↑/↓ or Ctrl+N/P, Enter execute, Tab add workflow, Ctrl+T workflow mode, Ctrl+C quit\n")
	b.WriteString("Workflow keys: n new, d/Ctrl+D delete, c clear, x/Enter execute, Ctrl+N/P switch, Ctrl+T search mode\n\n")

	if m.mode == modeSearch {
		b.WriteString("Mode: SEARCH\n")
		b.WriteString(fmt.Sprintf("Query: %s\n\n", m.query))

		if len(m.filtered) == 0 {
			b.WriteString("(no matched commands)\n")
		} else {
			limit := len(m.filtered)
			if limit > 14 {
				limit = 14
			}
			for idx, entry := range m.filtered[:limit] {
				prefix := "  "
				if idx == m.cursor {
					prefix = "> "
				}
				b.WriteString(fmt.Sprintf("%s%-28s %s\n", prefix, entry.Usage, entry.Description))
			}
		}
	} else {
		b.WriteString("Mode: WORKFLOW\n\n")
		for i, wf := range m.workflows {
			prefix := "  "
			if i == m.activeWF {
				prefix = "> "
			}
			b.WriteString(fmt.Sprintf("%sworkflow-%d (%d steps)\n", prefix, i+1, len(wf)))
			for _, step := range wf {
				b.WriteString("    - " + step + "\n")
			}
		}
	}

	return b.String()
}

func runInteractiveFlow(ctx context.Context, registry *Registry, store *StateStore) error {
	state, err := store.Load()
	if err != nil {
		return err
	}

	model := newInteractiveModel(buildInteractiveEntries(registry, state), state.Workflows)
	p := tea.NewProgram(model)
	resModel, err := p.Run()
	if err != nil {
		return err
	}

	finalModel, ok := resModel.(*interactiveModel)
	if !ok {
		return nil
	}

	if finalModel.dirty {
		state.Workflows = finalModel.workflows
		if err := store.Save(state); err != nil {
			return err
		}
	}

	switch finalModel.result.Action {
	case actionExecuteSelected:
		return executeInteractiveCommand(ctx, registry, state, finalModel.result.SelectedKey, finalModel.result.SelectedUse)
	case actionExecuteWorkflow:
		for _, step := range finalModel.result.Workflow {
			fmt.Printf("\n>>> workflow step: %s\n", step)
			if err := executeInteractiveCommand(ctx, registry, state, step, ""); err != nil {
				return fmt.Errorf("workflow step %q failed: %w", step, err)
			}
		}
	}

	return nil
}

func executeInteractiveCommand(ctx context.Context, registry *Registry, state *GGCState, key, usage string) error {
	entry, ok := registry.Get(key)
	if ok {
		if usage == "" {
			usage = entry.Usage
		}

		placeholders := parseUsagePlaceholders(usage)
		args := make([]string, 0, len(placeholders))
		for _, p := range placeholders {
			val := strings.TrimSpace(tap.Text(ctx, tap.TextOptions{
				Message:      fmt.Sprintf("%s:", p),
				Placeholder:  p,
				InitialValue: "",
				DefaultValue: "",
			}))
			if val == "" {
				return fmt.Errorf("%s is required", p)
			}
			args = append(args, val)
		}

		tokens := append(strings.Split(key, " "), args...)
		return registry.Execute(ctx, tokens)
	}

	alias, ok := state.Aliases[key]
	if !ok {
		return fmt.Errorf("unknown command key: %s", key)
	}

	args := make([]string, 0, alias.RequiredArgCount())
	for i := 0; i < alias.RequiredArgCount(); i++ {
		label := fmt.Sprintf("arg%d", i)
		val := strings.TrimSpace(tap.Text(ctx, tap.TextOptions{
			Message:      fmt.Sprintf("%s:", label),
			Placeholder:  label,
			InitialValue: "",
			DefaultValue: "",
		}))
		if val == "" {
			return fmt.Errorf("%s is required", label)
		}
		args = append(args, val)
	}

	return executeAlias(ctx, registry, state, alias, args, 1)
}

func parseUsagePlaceholders(usage string) []string {
	re := regexp.MustCompile(`<([^>]+)>`)
	matches := re.FindAllStringSubmatch(usage, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}
