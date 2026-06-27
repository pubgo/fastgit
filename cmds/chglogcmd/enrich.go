package chglogcmd

import (
	"fmt"
	"os"
	"strings"
)

var metaSections = []string{"影响范围", "验证建议", "回滚建议"}

// ReleaseMeta holds rule-based release note hints derived from a diff.
type ReleaseMeta struct {
	Impact     string
	Validation string
	Rollback   string
}

// DeriveReleaseMeta builds heuristic impact/validation/rollback notes from changed paths.
func DeriveReleaseMeta(diffNames string) ReleaseMeta {
	names := splitDiffNames(diffNames)
	if len(names) == 0 {
		return ReleaseMeta{
			Impact:     "暂无（未检测到文件变更）",
			Validation: "- 运行 `fastgit check run`",
			Rollback:   "- 关闭 PR 或 revert 相关 commit",
		}
	}

	return ReleaseMeta{
		Impact:     deriveImpact(names),
		Validation: deriveValidation(names),
		Rollback:   deriveRollback(names),
	}
}

func splitDiffNames(diffNames string) []string {
	lines := strings.Split(strings.TrimSpace(diffNames), "\n")
	var names []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "(no changes)" {
			names = append(names, line)
		}
	}
	return names
}

func deriveImpact(names []string) string {
	var areas []string
	seen := map[string]struct{}{}
	add := func(label string) {
		if _, ok := seen[label]; ok {
			return
		}
		seen[label] = struct{}{}
		areas = append(areas, label)
	}

	for _, name := range names {
		lower := strings.ToLower(name)
		switch {
		case strings.HasPrefix(name, "cmds/"):
			add("CLI 命令")
		case strings.HasPrefix(name, "pkg/"):
			add("核心库")
		case strings.HasPrefix(name, "docs/"):
			add("文档")
		case strings.Contains(lower, "migration"), strings.Contains(lower, "schema"):
			add("数据/迁移")
		case strings.Contains(lower, "auth"), strings.Contains(lower, "credential"), strings.Contains(lower, "secret"):
			add("安全/鉴权")
		case strings.HasSuffix(name, "_test.go"):
			add("测试")
		}
	}
	if len(areas) == 0 {
		return fmt.Sprintf("- 涉及 %d 个文件（请补充用户可见影响）", len(names))
	}
	var b strings.Builder
	for _, area := range areas {
		b.WriteString("- ")
		b.WriteString(area)
		b.WriteByte('\n')
	}
	b.WriteString(fmt.Sprintf("- 共 %d 个文件变更", len(names)))
	return strings.TrimSpace(b.String())
}

func deriveValidation(names []string) string {
	pkgs := map[string]struct{}{}
	for _, name := range names {
		if strings.HasSuffix(name, ".go") {
			if idx := strings.Index(name, "/"); idx > 0 {
				pkgs[name[:idx]] = struct{}{}
			}
		}
	}
	var b strings.Builder
	b.WriteString("- 运行 `fastgit check run`\n")
	if len(pkgs) > 0 {
		for pkg := range pkgs {
			b.WriteString("- 运行 `go test ./")
			b.WriteString(pkg)
			b.WriteString("/...`\n")
		}
	} else {
		b.WriteString("- 手动验证受影响功能路径\n")
	}
	return strings.TrimSpace(b.String())
}

func deriveRollback(names []string) string {
	for _, name := range names {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "migration") || strings.Contains(lower, "deploy") || strings.Contains(lower, "config") {
			return "- revert 相关 commit\n- 如涉及配置/迁移，按 runbook 回滚并验证数据一致性"
		}
	}
	return "- revert 相关 commit 或关闭 PR 不合并"
}

// ApplyReleaseMetaToUnreleased fills meta sections when they are still placeholder text.
func ApplyReleaseMetaToUnreleased(path string, meta ReleaseMeta) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	updated, changed := mergeMetaSections(string(content), meta)
	if !changed {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func mergeMetaSections(content string, meta ReleaseMeta) (string, bool) {
	sections := parseAllSections(content)
	changed := false
	apply := func(title, value string) {
		current := normalizeSectionBody(sections[title])
		if current != "暂无" && !strings.HasPrefix(current, "暂无（") {
			return
		}
		sections[title] = value
		changed = true
	}
	apply("影响范围", meta.Impact)
	apply("验证建议", meta.Validation)
	apply("回滚建议", meta.Rollback)
	if !changed {
		return content, false
	}
	return renderUnreleasedWithMeta(sections), true
}

func parseAllSections(content string) map[string]string {
	allTitles := append(append([]string{}, standardSections...), metaSections...)
	sections := make(map[string]string, len(allTitles))
	for _, title := range allTitles {
		sections[title] = "暂无"
	}

	current := ""
	var buf strings.Builder
	flush := func() {
		if current == "" {
			buf.Reset()
			return
		}
		sections[current] = normalizeSectionBody(buf.String())
		buf.Reset()
	}

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "## ") {
			flush()
			title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if containsString(allTitles, title) {
				current = title
			} else {
				current = ""
			}
			continue
		}
		if current == "" {
			continue
		}
		buf.WriteString(rawLine)
		buf.WriteByte('\n')
	}
	flush()
	return sections
}

func renderUnreleasedWithMeta(sections map[string]string) string {
	allTitles := append(append([]string{}, standardSections...), metaSections...)
	var b strings.Builder
	b.WriteString("# [Unreleased]\n\n")
	b.WriteString("> 推荐维护方式：`fastgit changelog draft|release`\n\n")
	for _, title := range allTitles {
		body := normalizeSectionBody(sections[title])
		b.WriteString("## ")
		b.WriteString(title)
		b.WriteString("\n\n")
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

// ValidateReleaseReadiness checks Unreleased content before release.
func ValidateReleaseReadiness(content string) error {
	sections := parseAllSections(content)
	if !hasMeaningfulEntries(sections) {
		return fmt.Errorf("unreleased.md 中没有可发布的变更条目")
	}
	for _, title := range metaSections {
		body := normalizeSectionBody(sections[title])
		if body == "暂无" {
			return fmt.Errorf("unreleased.md 缺少 %q 内容；运行 `fastgit changelog draft --enrich` 补充", title)
		}
	}
	return nil
}
