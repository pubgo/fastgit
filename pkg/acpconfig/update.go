package acpconfig

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AddAgent 在配置文件中写入/覆盖 acp.agents.<name>。
func AddAgent(name string, def AgentConfig) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("agent name is empty")
	}
	if strings.TrimSpace(def.Command) == "" {
		return fmt.Errorf("agent %q: command is empty", name)
	}
	return update(func(acpNode *yaml.Node) error {
		agents := getOrCreateMap(acpNode, "agents")
		mapSet(agents, name, agentConfigNode(def))
		return nil
	})
}

// RemoveAgent 从配置文件删除 acp.agents.<name>;若 default 指向它则一并清除。
func RemoveAgent(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("agent name is empty")
	}
	return update(func(acpNode *yaml.Node) error {
		agents := mapGet(acpNode, "agents")
		if agents == nil || agents.Kind != yaml.MappingNode {
			return fmt.Errorf("acp agent %q not found in config", name)
		}
		if !mapDelete(agents, name) {
			return fmt.Errorf("acp agent %q not found in config", name)
		}
		if d := mapGet(acpNode, "default"); d != nil && d.Value == name {
			// default 指向被删 agent 时清掉,避免悬空引用
			mapDelete(acpNode, "default")
		}
		return nil
	})
}

// SetDefault 设置 acp.default。name 为空时清除该字段。
func SetDefault(name string) error {
	name = strings.TrimSpace(name)
	if name != "" {
		cfg, err := Load()
		if err != nil {
			return err
		}
		if _, ok := cfg.Agents[name]; !ok {
			if _, inCatalog := Catalog[name]; !inCatalog {
				return fmt.Errorf("acp agent %q not found in config (acp.agents) or built-in catalog", name)
			}
		}
	}
	return update(func(acpNode *yaml.Node) error {
		if name == "" {
			mapDelete(acpNode, "default")
			return nil
		}
		mapSet(acpNode, "default", &yaml.Node{Kind: yaml.ScalarNode, Value: name, Tag: "!!str"})
		return nil
	})
}

// update 以 yaml.Node 方式改写配置文件的 acp: 段,保留其余内容与注释结构
// (yaml.Node 往返会保留注释;acp 段内被重建的部分注释可能丢失,可接受)。
func update(mutate func(acpNode *yaml.Node) error) error {
	path := ConfigPathFn()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var doc yaml.Node
	if strings.TrimSpace(string(data)) == "" {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{
			Kind: yaml.MappingNode, Tag: "!!map",
		}}}
	} else if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}

	root := &doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return fmt.Errorf("empty config document")
		}
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("config %s: top-level is not a mapping", path)
	}

	acpNode := getOrCreateMap(root, "acp")
	if err := mutate(acpNode); err != nil {
		return err
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func agentConfigNode(def AgentConfig) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapSet(node, "command", &yaml.Node{Kind: yaml.ScalarNode, Value: def.Command, Tag: "!!str"})
	if len(def.Args) > 0 {
		args := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, arg := range def.Args {
			args.Content = append(args.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: arg, Tag: "!!str"})
		}
		mapSet(node, "args", args)
	}
	if len(def.Env) > 0 {
		env := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for k, v := range def.Env {
			mapSet(env, k, &yaml.Node{Kind: yaml.ScalarNode, Value: v, Tag: "!!str"})
		}
		mapSet(node, "env", env)
	}
	return node
}

// map helpers over yaml mapping nodes (content is [k1,v1,k2,v2,...]).

func mapGet(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func mapSet(m *yaml.Node, key string, value *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = value
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		value,
	)
}

func mapDelete(m *yaml.Node, key string) bool {
	if m == nil || m.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return true
		}
	}
	return false
}

func getOrCreateMap(m *yaml.Node, key string) *yaml.Node {
	node := mapGet(m, key)
	if node == nil || node.Kind != yaml.MappingNode {
		node = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		mapSet(m, key, node)
	}
	return node
}
