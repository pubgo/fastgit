package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Entry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Dir         string `json:"dir"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
}

type CreateInput struct {
	Name     string
	BaseDir  string
	Force    bool
	Template string
	DirPerm  os.FileMode
	FilePerm os.FileMode
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type parsedSkill struct {
	Name        string
	Description string
	Source      string
}

func Discover(skillDirs []string) ([]Entry, []string) {
	warns := make([]string, 0)
	entries := make([]Entry, 0)
	seen := map[string]struct{}{}

	for _, root := range CompactStringSlice(skillDirs) {
		st, err := os.Stat(root)
		if err != nil {
			warns = append(warns, fmt.Sprintf("skill dir not available: %s (%v)", root, err))
			continue
		}
		if !st.IsDir() {
			warns = append(warns, fmt.Sprintf("skill path is not directory: %s", root))
			continue
		}
		children, err := os.ReadDir(root)
		if err != nil {
			warns = append(warns, fmt.Sprintf("read skill dir failed: %s (%v)", root, err))
			continue
		}
		for _, child := range children {
			if !child.IsDir() {
				continue
			}
			dirName := strings.TrimSpace(child.Name())
			if dirName == "" {
				continue
			}
			skillDir := filepath.Join(root, dirName)
			skillPath := filepath.Join(skillDir, "SKILL.md")
			if _, err := os.Stat(skillPath); err != nil {
				continue
			}
			parsed, err := ParseFile(skillPath, dirName)
			if err != nil {
				warns = append(warns, fmt.Sprintf("parse skill failed: %s (%v)", skillPath, err))
				continue
			}
			if parsed.Source == "frontmatter.name" && !strings.EqualFold(parsed.Name, dirName) {
				warns = append(warns, fmt.Sprintf("skill name mismatch: dir=%s frontmatter.name=%s", dirName, parsed.Name))
			}

			key := strings.ToLower(parsed.Name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			entries = append(entries, Entry{
				Name:        parsed.Name,
				Path:        skillPath,
				Dir:         skillDir,
				Description: parsed.Description,
				Source:      parsed.Source,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, warns
}

func FindByName(entries []Entry, name string) (Entry, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return Entry{}, fmt.Errorf("skill name is required")
	}
	for _, s := range entries {
		if strings.ToLower(strings.TrimSpace(s.Name)) == name {
			return s, nil
		}
	}
	return Entry{}, fmt.Errorf("skill not found: %s", name)
}

func ReadSkill(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("skill path is required")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read skill(%s): %w", path, err)
	}
	return string(content), nil
}

func CreateSkill(in CreateInput) (Entry, error) {
	name := SanitizeName(in.Name)
	if name == "" {
		return Entry{}, fmt.Errorf("invalid skill name")
	}

	baseDir := strings.TrimSpace(in.BaseDir)
	if baseDir == "" {
		baseDir = "./skills"
	}
	targetDir := filepath.Join(baseDir, name)
	targetFile := filepath.Join(targetDir, "SKILL.md")

	dirPerm := in.DirPerm
	if dirPerm == 0 {
		dirPerm = 0o755
	}
	filePerm := in.FilePerm
	if filePerm == 0 {
		filePerm = 0o644
	}
	if err := os.MkdirAll(targetDir, dirPerm); err != nil {
		return Entry{}, fmt.Errorf("mkdir skill dir(%s): %w", targetDir, err)
	}
	if !in.Force {
		if _, err := os.Stat(targetFile); err == nil {
			return Entry{}, fmt.Errorf("skill already exists: %s (use force to overwrite)", targetFile)
		}
	}

	tpl := strings.TrimSpace(in.Template)
	if tpl == "" {
		tpl = BuildTemplate(name)
	}
	if !strings.HasSuffix(tpl, "\n") {
		tpl += "\n"
	}

	if err := os.WriteFile(targetFile, []byte(tpl), filePerm); err != nil {
		return Entry{}, fmt.Errorf("write skill file(%s): %w", targetFile, err)
	}
	return Entry{Name: name, Path: targetFile, Dir: targetDir}, nil
}

func ExistingDirs(candidates []string) []string {
	out := make([]string, 0, len(candidates))
	for _, dir := range CompactStringSlice(candidates) {
		if DirExists(dir) {
			out = append(out, dir)
		}
	}
	return out
}

func DirExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func SanitizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.Trim(name, "-/")
	if name == "" {
		return ""
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return ""
	}
	return name
}

func BuildTemplate(name string) string {
	name = SanitizeName(name)
	return fmt.Sprintf(`---
name: %s
description: "Use when: 描述这个 skill 适用场景（关键词越具体越好）"
---

# %s

## 用途
- 简要描述这个 skill 负责的任务。

## 行为约束
- 不要臆造事实。
- 信息不足时先说明缺失上下文。

## 建议流程
1. 理解问题与边界。
2. 检索并确认相关文件。
3. 给出最小可执行方案。
`, name, name)
}

func ParseFile(path string, fallbackName string) (parsedSkill, error) {
	content, err := ReadSkill(path)
	if err != nil {
		return parsedSkill{}, err
	}
	return ParseContent(content, fallbackName)
}

func ParseContent(content, fallbackName string) (parsedSkill, error) {
	fallbackName = SanitizeName(fallbackName)
	if fallbackName == "" {
		fallbackName = "skill"
	}

	result := parsedSkill{Name: fallbackName, Source: "directory"}
	body := strings.TrimSpace(content)
	if body == "" {
		return result, nil
	}

	fm, rest, hasFM, err := splitFrontmatter(body)
	if err != nil {
		return parsedSkill{}, err
	}
	if hasFM {
		if strings.TrimSpace(fm.Name) != "" {
			n := SanitizeName(fm.Name)
			if n == "" {
				return parsedSkill{}, fmt.Errorf("invalid frontmatter name: %q", fm.Name)
			}
			result.Name = n
			result.Source = "frontmatter.name"
		}
		if strings.TrimSpace(fm.Description) != "" {
			result.Description = strings.TrimSpace(fm.Description)
		}
		body = rest
	}

	heading := extractTopHeading(body)
	if heading != "" {
		h := SanitizeName(heading)
		if h != "" && result.Source == "directory" {
			result.Name = h
			result.Source = "heading"
		}
	}

	return result, nil
}

func splitFrontmatter(content string) (frontmatter, string, bool, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return frontmatter{}, content, false, nil
	}
	if strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{}, content, false, nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return frontmatter{}, "", true, fmt.Errorf("frontmatter start found but closing --- missing")
	}

	raw := strings.Join(lines[1:end], "\n")
	var fm frontmatter
	if strings.TrimSpace(raw) != "" {
		if err := yaml.Unmarshal([]byte(raw), &fm); err != nil {
			return frontmatter{}, "", true, fmt.Errorf("invalid frontmatter yaml: %w", err)
		}
	}
	rest := strings.Join(lines[end+1:], "\n")
	return fm, strings.TrimSpace(rest), true, nil
}

func extractTopHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
		if strings.HasPrefix(line, "##") {
			return ""
		}
		if strings.HasPrefix(line, "---") {
			continue
		}
		return ""
	}
	return ""
}

func CompactStringSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
