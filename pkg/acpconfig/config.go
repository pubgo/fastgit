// Package acpconfig 管理 fastgit 配置文件中的 acp: 段:
// 读取、解析、选择默认 agent,以及供 `ggc acp` 子命令使用的写回操作。
package acpconfig

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pubgo/fastgit/configs"
	"github.com/pubgo/fastgit/pkg/acpclient"
)

// EnvAgentVar 用于临时覆盖选用的 ACP agent 名。
const EnvAgentVar = "FASTGIT_ACP_AGENT"

// ConfigPathFn resolves the fastgit config file path. Tests override it to
// point at a temp file (xdg paths are fixed at package init in production).
var ConfigPathFn = configs.GetConfigPath

// AgentConfig 是配置文件中单个 acp agent 的定义。
type AgentConfig struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// Config 对应配置文件中的 acp: 段。
type Config struct {
	Default string                 `yaml:"default,omitempty"`
	Agents  map[string]AgentConfig `yaml:"agents,omitempty"`
}

// Catalog 是内置的常见 ACP agent 命令,`ggc acp add <name>` 可直接引用。
var Catalog = map[string]AgentConfig{
	"claude":   {Command: "npx", Args: []string{"-y", "@zed-industries/claude-code-acp"}},
	"codex":    {Command: "npx", Args: []string{"-y", "@zed-industries/codex-acp"}},
	"opencode": {Command: "opencode", Args: []string{"acp"}},
	"gemini":   {Command: "gemini", Args: []string{"--experimental-acp"}},
}

// Load 从 fastgit 配置文件读取 acp 段;文件或段不存在时返回空配置。
func Load() (*Config, error) {
	cfg := &Config{Agents: map[string]AgentConfig{}}
	data, err := os.ReadFile(ConfigPathFn())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	var file struct {
		Acp *Config `yaml:"acp"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse acp config: %w", err)
	}
	if file.Acp != nil {
		if file.Acp.Default != "" {
			cfg.Default = file.Acp.Default
		}
		for name, agent := range file.Acp.Agents {
			cfg.Agents[name] = agent
		}
	}
	return cfg, nil
}

// Resolve picks the agent to use. Precedence: explicit name (want), then
// FASTGIT_ACP_AGENT env, then config default, then the only configured agent,
// then an installed catalog agent.
func (c *Config) Resolve(want string) (acpclient.Agent, error) {
	names := []string{}
	if w := strings.TrimSpace(want); w != "" {
		names = append(names, w)
	}
	if e := strings.TrimSpace(os.Getenv(EnvAgentVar)); e != "" {
		names = append(names, e)
	}
	if d := strings.TrimSpace(c.Default); d != "" {
		names = append(names, d)
	}

	for _, name := range names {
		if agent, ok := c.agentByName(name); ok {
			return agent, nil
		}
		return acpclient.Agent{}, fmt.Errorf("acp agent %q not found in config (acp.agents) or built-in catalog", name)
	}

	if len(c.Agents) == 1 {
		for name, def := range c.Agents {
			return acpclient.Agent{Name: name, Command: def.Command, Args: def.Args, Env: def.Env}, nil
		}
	}

	// 未配置时探测 catalog 中已安装的 agent(按稳定顺序)。
	for _, name := range sortedCatalogNames() {
		if agent, ok := c.agentByName(name); ok {
			return agent, nil
		}
	}

	if len(names) == 0 && len(c.Agents) == 0 {
		return acpclient.Agent{}, fmt.Errorf("no acp agent configured; run `ggc acp add <name>` or set acp.default in %s", ConfigPathFn())
	}
	return acpclient.Agent{}, fmt.Errorf("no usable acp agent found; run `ggc acp list` to see candidates")
}

func (c *Config) agentByName(name string) (acpclient.Agent, bool) {
	if def, ok := c.Agents[name]; ok && strings.TrimSpace(def.Command) != "" {
		return acpclient.Agent{Name: name, Command: def.Command, Args: def.Args, Env: def.Env}, true
	}
	if def, ok := Catalog[name]; ok && HasExecutable(def.Command) {
		return acpclient.Agent{Name: name, Command: def.Command, Args: def.Args, Env: def.Env}, true
	}
	return acpclient.Agent{}, false
}

// HasExecutable reports whether command resolves on PATH.
func HasExecutable(command string) bool {
	_, err := exec.LookPath(strings.TrimSpace(command))
	return err == nil
}

func sortedCatalogNames() []string {
	names := make([]string, 0, len(Catalog))
	for name := range Catalog {
		names = append(names, name)
	}
	// 小列表,插入排序即可。
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}
