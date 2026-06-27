package chglogcmd

import (
	"fmt"
	"strings"
)

// ValidateBumpConsistency checks whether --bump matches Unreleased section content.
func ValidateBumpConsistency(sections map[string]string, bump string) error {
	bump = strings.ToLower(strings.TrimSpace(bump))
	if bump == "" {
		return nil
	}

	analysis := analyzeReleaseSections(sections)
	switch bump {
	case "patch":
		if analysis.hasBreaking {
			return fmt.Errorf("breaking changes detected; use --bump major instead of patch")
		}
		if analysis.hasAdded && !analysis.hasFixed && !analysis.hasChanged {
			return fmt.Errorf("新增 section has entries; prefer --bump minor (or --skip-bump-check)")
		}
	case "minor":
		if analysis.hasBreaking {
			return fmt.Errorf("breaking changes detected; use --bump major instead of minor")
		}
		if analysis.docsOnly {
			return fmt.Errorf("docs-only changes; prefer --bump patch (or --skip-bump-check)")
		}
	case "major":
		if analysis.docsOnly {
			return fmt.Errorf("docs-only changes; major bump is unusual (use --skip-bump-check to override)")
		}
	default:
		return fmt.Errorf("unsupported bump level: %s", bump)
	}
	return nil
}

type releaseAnalysis struct {
	hasAdded    bool
	hasFixed    bool
	hasChanged  bool
	hasDocs     bool
	hasBreaking bool
	docsOnly    bool
}

func analyzeReleaseSections(sections map[string]string) releaseAnalysis {
	a := releaseAnalysis{}
	a.hasAdded = sectionHasEntries(sections["新增"])
	a.hasFixed = sectionHasEntries(sections["修复"])
	a.hasChanged = sectionHasEntries(sections["变更"])
	a.hasDocs = sectionHasEntries(sections["文档"])
	a.docsOnly = a.hasDocs && !a.hasAdded && !a.hasFixed && !a.hasChanged

	for _, title := range standardSections {
		body := strings.ToLower(normalizeSectionBody(sections[title]))
		if containsBreakingKeyword(body) {
			a.hasBreaking = true
			break
		}
	}
	return a
}

func sectionHasEntries(body string) bool {
	body = strings.TrimSpace(normalizeSectionBody(body))
	if body == "" || body == "暂无" {
		return false
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if line != "" && line != "暂无" {
			return true
		}
	}
	return false
}

func containsBreakingKeyword(text string) bool {
	keywords := []string{
		"breaking change",
		"breaking",
		"不兼容",
		"破坏性",
		"重大变更",
		"breaking:",
	}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
