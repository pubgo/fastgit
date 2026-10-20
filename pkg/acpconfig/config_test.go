package acpconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// useTempConfig points ConfigPathFn at a per-test file so production config
// is never touched.
func useTempConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	old := ConfigPathFn
	ConfigPathFn = func() string { return path }
	t.Cleanup(func() { ConfigPathFn = old })
	return path
}

func TestLoadMissingFile(t *testing.T) {
	useTempConfig(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "", cfg.Default)
	require.Empty(t, cfg.Agents)
}

func TestAddLoadResolve(t *testing.T) {
	path := useTempConfig(t)
	require.NoError(t, os.WriteFile(path, []byte("version:\n  name: v0.0.3\n"), 0o644))
	require.NoError(t, AddAgent("myagent", AgentConfig{Command: "sh", Args: []string{"-c", "exit 0"}, Env: map[string]string{"K": "V"}}))

	cfg, err := Load()
	require.NoError(t, err)
	require.Contains(t, cfg.Agents, "myagent")

	agent, err := cfg.Resolve("myagent")
	require.NoError(t, err)
	require.Equal(t, "sh", agent.Command)
	require.Equal(t, []string{"-c", "exit 0"}, agent.Args)
	require.Equal(t, map[string]string{"K": "V"}, agent.Env)

	_, err = cfg.Resolve("not-there")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not-there")
}

func TestUpdatePreservesOtherSections(t *testing.T) {
	path := useTempConfig(t)
	require.NoError(t, os.WriteFile(path, []byte("version:\n  name: v0.0.3\nopenai:\n  api_key: sk-test\n"), 0o644))
	require.NoError(t, AddAgent("a1", AgentConfig{Command: "echo"}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	body := string(data)
	require.Contains(t, body, "api_key: sk-test")
	require.Contains(t, body, "v0.0.3")
	require.Contains(t, body, "acp:")
}

func TestSetDefaultAndRemove(t *testing.T) {
	path := useTempConfig(t)
	require.NoError(t, os.WriteFile(path, []byte("version:\n  name: v0.0.3\n"), 0o644))
	require.NoError(t, AddAgent("a1", AgentConfig{Command: "echo"}))
	require.NoError(t, SetDefault("a1"))

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "a1", cfg.Default)

	require.Error(t, SetDefault("ghost"))

	require.NoError(t, RemoveAgent("a1"))
	cfg, err = Load()
	require.NoError(t, err)
	require.Empty(t, cfg.Agents)
	require.Equal(t, "", cfg.Default) // default cleared with the agent

	require.Error(t, RemoveAgent("a1")) // gone already
}

func TestResolvePrecedence(t *testing.T) {
	path := useTempConfig(t)
	require.NoError(t, os.WriteFile(path, []byte("acp:\n  default: a1\n  agents:\n    a1:\n      command: echo\n    a2:\n      command: true\n"), 0o644))
	cfg, err := Load()
	require.NoError(t, err)

	t.Setenv(EnvAgentVar, "")
	agent, err := cfg.Resolve("")
	require.NoError(t, err)
	require.Equal(t, "a1", agent.Name)

	agent, err = cfg.Resolve("a2")
	require.NoError(t, err)
	require.Equal(t, "a2", agent.Name)

	t.Setenv(EnvAgentVar, "a2")
	agent, err = cfg.Resolve("")
	require.NoError(t, err)
	require.Equal(t, "a2", agent.Name)
}

func TestResolveSingleConfiguredAgent(t *testing.T) {
	path := useTempConfig(t)
	require.NoError(t, os.WriteFile(path, []byte("acp:\n  agents:\n    only:\n      command: echo\n"), 0o644))
	cfg, err := Load()
	require.NoError(t, err)
	agent, err := cfg.Resolve("")
	require.NoError(t, err)
	require.Equal(t, "only", agent.Name)
}

func TestResolveNoConfigAndNoCatalog(t *testing.T) {
	path := useTempConfig(t)
	require.NoError(t, os.WriteFile(path, []byte("version:\n  name: v0.0.3\n"), 0o644))
	t.Setenv(EnvAgentVar, "")
	cfg, err := Load()
	require.NoError(t, err)

	_, err = cfg.Resolve("")
	if hasAnyCatalogExecutable() {
		// machine has one of the catalog commands installed; resolution succeeds
		return
	}
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "ggc acp add"))
}

func hasAnyCatalogExecutable() bool {
	for _, def := range Catalog {
		if HasExecutable(def.Command) {
			return true
		}
	}
	return false
}
