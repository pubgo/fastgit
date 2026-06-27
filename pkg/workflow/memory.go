package workflow

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pubgo/fastgit/configs"
	"gopkg.in/yaml.v3"
)

// Memory stores command transition frequencies for next-step recommendations.
type Memory struct {
	path string
	data State
}

// State is persisted workflow memory.
type State struct {
	Enabled     bool                       `yaml:"enabled"`
	Transitions map[string]map[string]int  `yaml:"transitions,omitempty"`
	LastCommand string                     `yaml:"last_command,omitempty"`
}

// NewMemory loads workflow memory from the fastgit config directory.
func NewMemory() (*Memory, error) {
	path := filepath.Join(filepath.Dir(configs.GetConfigPath()), "workflow.yaml")
	m := &Memory{path: path, data: defaultState()}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

func defaultState() State {
	return State{Enabled: true, Transitions: map[string]map[string]int{}}
}

func (m *Memory) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return yaml.Unmarshal(data, &m.data)
}

// Save persists workflow memory.
func (m *Memory) Save() error {
	if m.data.Transitions == nil {
		m.data.Transitions = map[string]map[string]int{}
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(m.data)
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, raw, 0o644)
}

// Record stores a completed command name.
func (m *Memory) Record(command string) error {
	command = normalizeCommand(command)
	if command == "" || !m.data.Enabled {
		return nil
	}
	if m.data.Transitions == nil {
		m.data.Transitions = map[string]map[string]int{}
	}
	if prev := strings.TrimSpace(m.data.LastCommand); prev != "" && prev != command {
		if m.data.Transitions[prev] == nil {
			m.data.Transitions[prev] = map[string]int{}
		}
		m.data.Transitions[prev][command]++
	}
	m.data.LastCommand = command
	return m.Save()
}

// Recommend returns suggested next commands after the given command.
func (m *Memory) Recommend(command string) []string {
	command = normalizeCommand(command)
	if command == "" {
		return nil
	}

	seen := map[string]struct{}{}
	var out []string
	appendUnique := func(items ...string) {
		for _, item := range items {
			item = normalizeCommand(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}

	if m != nil && m.data.Enabled {
		counts := m.data.Transitions[command]
		type pair struct {
			cmd   string
			count int
		}
		pairs := make([]pair, 0, len(counts))
		for cmd, count := range counts {
			pairs = append(pairs, pair{cmd: cmd, count: count})
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].count == pairs[j].count {
				return pairs[i].cmd < pairs[j].cmd
			}
			return pairs[i].count > pairs[j].count
		})
		for _, p := range pairs {
			appendUnique(p.cmd)
		}
	}

	appendUnique(defaultRecommendations[command]...)
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

var defaultRecommendations = map[string][]string{
	"commit":  {"push", "check run", "pr create"},
	"push":    {"pr create", "pr status"},
	"pull":    {"conflict summary", "check run"},
	"check":   {"commit", "push"},
	"pr":      {"changelog draft", "pr status"},
	"rebase":  {"conflict summary", "check run"},
	"merge":   {"check run", "push"},
	"changelog": {"pr create", "tag"},
}

func normalizeCommand(command string) string {
	command = strings.TrimSpace(strings.ToLower(command))
	command = strings.TrimPrefix(command, "fastgit ")
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	if parts[0] == "check" && len(parts) > 1 {
		return "check"
	}
	if parts[0] == "pr" {
		return "pr"
	}
	return parts[0]
}

// LastCommandName returns the most recently recorded command.
func (m *Memory) LastCommandName() string {
	if m == nil {
		return ""
	}
	return normalizeCommand(m.data.LastCommand)
}

// RecommendFor returns next-step suggestions after a command name.
func RecommendFor(command string) []string {
	mem, err := NewMemory()
	if err != nil {
		return nil
	}
	return mem.Recommend(normalizeCommand(command))
}

// FormatHint renders recommendations for CLI output.
func FormatHint(command string, recommendations []string) string {
	if len(recommendations) == 0 {
		return ""
	}
	return "Next: " + strings.Join(recommendations, " → ")
}
