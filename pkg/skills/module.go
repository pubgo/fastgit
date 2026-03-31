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
	ID          string         `json:"id,omitempty"`
	Kind        string         `json:"kind,omitempty"`
	Namespace   string         `json:"namespace,omitempty"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug,omitempty"`
	DisplayName string         `json:"displayName,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Version     string         `json:"version,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	UseWhen     []string       `json:"useWhen,omitempty"`
	Tools       []string       `json:"tools,omitempty"`
	Path        string         `json:"path"`
	Dir         string         `json:"dir"`
	Description string         `json:"description,omitempty"`
	Title       string         `json:"title,omitempty"`
	H2          []string       `json:"h2,omitempty"`
	H3          []string       `json:"h3,omitempty"`
	Sections    []Section      `json:"sections,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Source      string         `json:"source,omitempty"`
}

type Section struct {
	Title       string       `json:"title"`
	Content     string       `json:"content,omitempty"`
	Subsections []Subsection `json:"subsections,omitempty"`
}

type Subsection struct {
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
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
	Summary     string `yaml:"summary"`
}

type ParsedSkill struct {
	Name        string
	Slug        string
	Description string
	Summary     string
	Version     string
	Tags        []string
	UseWhen     []string
	Tools       []string
	Title       string
	H2          []string
	H3          []string
	Sections    []Section
	Metadata    map[string]any
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
				ID:          buildSkillID(root, parsed.Name),
				Kind:        "local",
				Namespace:   strings.TrimSpace(root),
				Name:        parsed.Name,
				Slug:        parsed.Slug,
				DisplayName: parsed.Title,
				Summary:     parsed.Summary,
				Version:     parsed.Version,
				Tags:        parsed.Tags,
				UseWhen:     parsed.UseWhen,
				Tools:       parsed.Tools,
				Path:        skillPath,
				Dir:         skillDir,
				Description: parsed.Description,
				Title:       parsed.Title,
				H2:          parsed.H2,
				H3:          parsed.H3,
				Sections:    parsed.Sections,
				Metadata:    parsed.Metadata,
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
	return Entry{
		ID:        buildSkillID(baseDir, name),
		Kind:      "local",
		Namespace: strings.TrimSpace(baseDir),
		Name:      name,
		Slug:      name,
		Path:      targetFile,
		Dir:       targetDir,
	}, nil
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
summary: "%s 技能摘要"
description: "Use when: 描述这个 skill 适用场景（关键词越具体越好）"
version: "0.1.0"
tags: ["repo", "workflow"]
use_when: ["当你需要处理该类任务时"]
tools: ["skills_tool"]
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
`, name, name, name)
}

func ParseFile(path string, fallbackName string) (ParsedSkill, error) {
	content, err := ReadSkill(path)
	if err != nil {
		return ParsedSkill{}, err
	}
	return ParseContent(content, fallbackName)
}

func ParseContent(content, fallbackName string) (ParsedSkill, error) {
	fallbackName = SanitizeName(fallbackName)
	if fallbackName == "" {
		fallbackName = "skill"
	}

	result := ParsedSkill{Name: fallbackName, Slug: fallbackName, Source: "directory", Metadata: map[string]any{}}
	body := strings.TrimSpace(content)
	if body == "" {
		return result, nil
	}

	fm, fmRaw, rest, hasFM, err := splitFrontmatter(body)
	if err != nil {
		return ParsedSkill{}, err
	}
	if hasFM {
		meta, err := parseFrontmatterMap(fmRaw)
		if err != nil {
			return ParsedSkill{}, err
		}
		result.Metadata = meta
		result.Summary = strings.TrimSpace(fm.Summary)
		if strings.TrimSpace(fm.Name) != "" {
			n := SanitizeName(fm.Name)
			if n == "" {
				return ParsedSkill{}, fmt.Errorf("invalid frontmatter name: %q", fm.Name)
			}
			result.Name = n
			result.Slug = n
			result.Source = "frontmatter.name"
		}
		if strings.TrimSpace(fm.Description) != "" {
			result.Description = strings.TrimSpace(fm.Description)
		}
		spec := extractSkillSpec(meta)
		if result.Summary == "" {
			result.Summary = spec.Summary
		}
		if result.Description == "" {
			result.Description = spec.Summary
		}
		result.Version = spec.Version
		result.Tags = spec.Tags
		result.UseWhen = spec.UseWhen
		result.Tools = spec.Tools
		body = rest
	}

	title, h2, h3, sections := extractHeadings(body)
	if title != "" {
		result.Title = title
		h := SanitizeName(title)
		if h != "" && result.Source == "directory" {
			result.Name = h
			result.Slug = h
			result.Source = "heading"
		}
	}
	result.H2 = h2
	result.H3 = h3
	result.Sections = sections

	return result, nil
}

func splitFrontmatter(content string) (frontmatter, string, string, bool, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return frontmatter{}, "", content, false, nil
	}
	if strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{}, "", content, false, nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return frontmatter{}, "", "", true, fmt.Errorf("frontmatter start found but closing --- missing")
	}

	raw := strings.Join(lines[1:end], "\n")
	var fm frontmatter
	if strings.TrimSpace(raw) != "" {
		if err := yaml.Unmarshal([]byte(raw), &fm); err != nil {
			return frontmatter{}, raw, "", true, fmt.Errorf("invalid frontmatter yaml: %w", err)
		}
	}
	rest := strings.Join(lines[end+1:], "\n")
	return fm, raw, strings.TrimSpace(rest), true, nil
}

func extractHeadings(content string) (title string, h2 []string, h3 []string, sections []Section) {
	h2 = make([]string, 0)
	h3 = make([]string, 0)
	sections = make([]Section, 0)

	currentH2 := -1
	currentH3 := -1

	appendContent := func(line string) {
		if currentH2 < 0 {
			return
		}
		if currentH3 >= 0 {
			if sections[currentH2].Subsections[currentH3].Content == "" {
				sections[currentH2].Subsections[currentH3].Content = line
			} else {
				sections[currentH2].Subsections[currentH3].Content += "\n" + line
			}
			return
		}
		if sections[currentH2].Content == "" {
			sections[currentH2].Content = line
		} else {
			sections[currentH2].Content += "\n" + line
		}
	}

	for _, line := range strings.Split(content, "\n") {
		raw := line
		line = strings.TrimSpace(raw)
		if line == "" {
			appendContent("")
			continue
		}
		if strings.HasPrefix(line, "# ") {
			if title == "" {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
			continue
		}
		if strings.HasPrefix(line, "### ") {
			label := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if label != "" {
				if currentH2 < 0 {
					sections = append(sections, Section{Title: "ungrouped"})
					h2 = append(h2, "ungrouped")
					currentH2 = len(sections) - 1
				}
				sections[currentH2].Subsections = append(sections[currentH2].Subsections, Subsection{Title: label})
				currentH3 = len(sections[currentH2].Subsections) - 1
				h3 = append(h3, label)
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			label := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if label != "" {
				sections = append(sections, Section{Title: label})
				currentH2 = len(sections) - 1
				currentH3 = -1
				h2 = append(h2, label)
			}
			continue
		}
		if strings.HasPrefix(line, "---") {
			continue
		}
		appendContent(raw)
	}

	for i := range sections {
		sections[i].Content = strings.TrimSpace(sections[i].Content)
		for j := range sections[i].Subsections {
			sections[i].Subsections[j].Content = strings.TrimSpace(sections[i].Subsections[j].Content)
		}
	}

	return title, h2, h3, sections
}

func FindSectionContent(sections []Section, headings ...string) (string, bool) {
	if len(headings) == 0 {
		return "", false
	}
	h2Key := strings.TrimSpace(headings[0])
	if h2Key == "" {
		return "", false
	}

	for _, sec := range sections {
		if !strings.EqualFold(strings.TrimSpace(sec.Title), h2Key) {
			continue
		}
		if len(headings) == 1 {
			return sec.Content, true
		}
		h3Key := strings.TrimSpace(headings[1])
		for _, sub := range sec.Subsections {
			if strings.EqualFold(strings.TrimSpace(sub.Title), h3Key) {
				return sub.Content, true
			}
		}
		return "", false
	}

	return "", false
}

func parseFrontmatterMap(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}
	var meta map[string]any
	if err := yaml.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("invalid frontmatter yaml: %w", err)
	}
	if meta == nil {
		return map[string]any{}, nil
	}
	return meta, nil
}

type skillSpec struct {
	Summary string
	Version string
	Tags    []string
	UseWhen []string
	Tools   []string
}

func extractSkillSpec(meta map[string]any) skillSpec {
	if meta == nil {
		return skillSpec{}
	}
	return skillSpec{
		Summary: firstString(meta, "summary", "brief", "abstract"),
		Version: firstString(meta, "version", "skill_version"),
		Tags:    firstStringSlice(meta, "tags", "keywords"),
		UseWhen: firstStringSlice(meta, "use_when", "useWhen", "when"),
		Tools:   firstStringSlice(meta, "tools", "available_tools", "tool_allowlist"),
	}
}

func firstString(meta map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := meta[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func firstStringSlice(meta map[string]any, keys ...string) []string {
	for _, k := range keys {
		if v, ok := meta[k]; ok {
			items := toStringSlice(v)
			if len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func toStringSlice(v any) []string {
	switch tv := v.(type) {
	case string:
		trimmed := strings.TrimSpace(tv)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	case []string:
		return CompactStringSlice(tv)
	case []any:
		out := make([]string, 0, len(tv))
		for _, item := range tv {
			s := strings.TrimSpace(fmt.Sprint(item))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return CompactStringSlice(out)
	default:
		s := strings.TrimSpace(fmt.Sprint(tv))
		if s == "" || s == "<nil>" {
			return nil
		}
		return []string{s}
	}
}

func buildSkillID(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = SanitizeName(name)
	if namespace == "" {
		namespace = "local"
	}
	if name == "" {
		name = "skill"
	}
	return namespace + ":" + name
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
