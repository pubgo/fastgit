package ggccmd

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/shell"
)

var placeholderRe = regexp.MustCompile(`\{(\d+)\}`)

type AliasDef struct {
	Single   string
	Sequence []string
}

func (a *AliasDef) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		a.Single = strings.TrimSpace(value.Value)
		a.Sequence = nil
		return nil
	case yaml.SequenceNode:
		a.Single = ""
		a.Sequence = make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("alias sequence must contain strings")
			}
			a.Sequence = append(a.Sequence, strings.TrimSpace(item.Value))
		}
		return nil
	default:
		return fmt.Errorf("alias must be string or string list")
	}
}

func (a AliasDef) MarshalYAML() (interface{}, error) {
	if len(a.Sequence) > 0 {
		return a.Sequence, nil
	}
	return a.Single, nil
}

func (a AliasDef) Templates() []string {
	if len(a.Sequence) > 0 {
		out := make([]string, len(a.Sequence))
		copy(out, a.Sequence)
		return out
	}
	if strings.TrimSpace(a.Single) == "" {
		return nil
	}
	return []string{a.Single}
}

func (a AliasDef) MaxPlaceholderIndex() int {
	maxIdx := -1
	for _, tpl := range a.Templates() {
		matches := placeholderRe.FindAllStringSubmatch(tpl, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			idx, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			if idx > maxIdx {
				maxIdx = idx
			}
		}
	}
	return maxIdx
}

func (a AliasDef) RequiredArgCount() int {
	return a.MaxPlaceholderIndex() + 1
}

func (a AliasDef) UsageSuffix() string {
	count := a.RequiredArgCount()
	if count <= 0 {
		return ""
	}
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, fmt.Sprintf("<arg%d>", i))
	}
	return " " + strings.Join(parts, " ")
}

func (a AliasDef) Summary() string {
	tpls := a.Templates()
	if len(tpls) == 0 {
		return "(empty alias)"
	}
	return strings.Join(tpls, " -> ")
}

func expandAliasTemplate(tpl string, args []string) (string, error) {
	missing := -1
	expanded := placeholderRe.ReplaceAllStringFunc(tpl, func(match string) string {
		parts := placeholderRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		idx, err := strconv.Atoi(parts[1])
		if err != nil {
			return match
		}
		if idx < 0 || idx >= len(args) {
			missing = idx
			return match
		}
		return args[idx]
	})
	if missing >= 0 {
		return "", fmt.Errorf("missing alias argument {%d}", missing)
	}
	return expanded, nil
}

func splitCommandLine(line string) ([]string, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	fields, err := shell.Fields(line, nil)
	if err == nil {
		return fields, nil
	}

	// fallback
	return strings.Fields(line), nil
}

func sortedAliasNames(m map[string]AliasDef) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
