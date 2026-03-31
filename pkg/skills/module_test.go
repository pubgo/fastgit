package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseContent_FrontmatterPreferred(t *testing.T) {
	content := `---
name: go-review
description: "Use when: review go changes"
---

# should-be-ignored
`

	parsed, err := ParseContent(content, "fallback")
	if err != nil {
		t.Fatalf("ParseContent returned error: %v", err)
	}
	if parsed.Name != "go-review" {
		t.Fatalf("expected name go-review, got %s", parsed.Name)
	}
	if parsed.Description != "Use when: review go changes" {
		t.Fatalf("unexpected description: %s", parsed.Description)
	}
	if parsed.Source != "frontmatter.name" {
		t.Fatalf("expected source frontmatter.name, got %s", parsed.Source)
	}
}

func TestParseContent_FallbackToHeading(t *testing.T) {
	content := `# Repo Context

something`

	parsed, err := ParseContent(content, "fallback")
	if err != nil {
		t.Fatalf("ParseContent returned error: %v", err)
	}
	if parsed.Name != "repo-context" {
		t.Fatalf("expected name repo-context, got %s", parsed.Name)
	}
	if parsed.Source != "heading" {
		t.Fatalf("expected source heading, got %s", parsed.Source)
	}
}

func TestParseContent_InvalidFrontmatterName(t *testing.T) {
	content := `---
name: "bad name !"
---

# test`

	_, err := ParseContent(content, "fallback")
	if err == nil {
		t.Fatalf("expected error for invalid frontmatter name")
	}
}

func TestParseContent_MetadataAndHeadingHierarchy(t *testing.T) {
	content := `---
name: runtime-bridge
description: "Use when: building runtime adapter"
owner: infra
tags:
  - runtime
  - adapter
---

# Runtime Bridge

## Scope
text

### Step 1
text

### Step 2
text

## Notes
`

	parsed, err := ParseContent(content, "fallback")
	if err != nil {
		t.Fatalf("ParseContent returned error: %v", err)
	}
	if parsed.Name != "runtime-bridge" {
		t.Fatalf("expected name runtime-bridge, got %s", parsed.Name)
	}
	if parsed.Title != "Runtime Bridge" {
		t.Fatalf("expected title Runtime Bridge, got %s", parsed.Title)
	}
	if len(parsed.H2) != 2 || parsed.H2[0] != "Scope" || parsed.H2[1] != "Notes" {
		t.Fatalf("unexpected h2 headings: %#v", parsed.H2)
	}
	if len(parsed.H3) != 2 || parsed.H3[0] != "Step 1" || parsed.H3[1] != "Step 2" {
		t.Fatalf("unexpected h3 headings: %#v", parsed.H3)
	}
	if parsed.Metadata["owner"] != "infra" {
		t.Fatalf("expected metadata owner=infra, got %#v", parsed.Metadata["owner"])
	}
	if parsed.Source != "frontmatter.name" {
		t.Fatalf("expected source frontmatter.name, got %s", parsed.Source)
	}
	if len(parsed.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(parsed.Sections))
	}
	if parsed.Sections[0].Title != "Scope" {
		t.Fatalf("expected first section Scope, got %s", parsed.Sections[0].Title)
	}
	if len(parsed.Sections[0].Subsections) != 2 {
		t.Fatalf("expected 2 subsections under Scope, got %d", len(parsed.Sections[0].Subsections))
	}
	if parsed.Sections[0].Subsections[0].Title != "Step 1" {
		t.Fatalf("expected first subsection Step 1, got %s", parsed.Sections[0].Subsections[0].Title)
	}
}

func TestFindSectionContent(t *testing.T) {
	content := `# Skill Title

## Scope
scope line 1
scope line 2

### Step 1
step1 content

### Step 2
step2 content

## Notes
notes content`

	parsed, err := ParseContent(content, "fallback")
	if err != nil {
		t.Fatalf("ParseContent returned error: %v", err)
	}

	scope, ok := FindSectionContent(parsed.Sections, "Scope")
	if !ok || !strings.Contains(scope, "scope line 1") {
		t.Fatalf("expected to find Scope content, got ok=%v content=%q", ok, scope)
	}

	step2, ok := FindSectionContent(parsed.Sections, "Scope", "Step 2")
	if !ok || !strings.Contains(step2, "step2 content") {
		t.Fatalf("expected to find Step 2 content, got ok=%v content=%q", ok, step2)
	}

	_, ok = FindSectionContent(parsed.Sections, "Scope", "Unknown")
	if ok {
		t.Fatalf("expected not found for unknown subsection")
	}
}

func TestDiscover_HeadingAndFrontmatterExtraction(t *testing.T) {
	root := t.TempDir()

	headingDir := filepath.Join(root, "heading-only")
	if err := os.MkdirAll(headingDir, 0o755); err != nil {
		t.Fatalf("mkdir heading dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(headingDir, "SKILL.md"), []byte(`# Repo Context

plain heading skill`), 0o644); err != nil {
		t.Fatalf("write heading skill: %v", err)
	}

	frontmatterDir := filepath.Join(root, "frontmatter-skill")
	if err := os.MkdirAll(frontmatterDir, 0o755); err != nil {
		t.Fatalf("mkdir frontmatter dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(frontmatterDir, "SKILL.md"), []byte(`---
name: go-review
description: "Use when: review go changes"
---

# ignored-heading`), 0o644); err != nil {
		t.Fatalf("write frontmatter skill: %v", err)
	}

	brokenDir := filepath.Join(root, "broken-skill")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("mkdir broken dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "SKILL.md"), []byte("---\nname: bad\n"), 0o644); err != nil {
		t.Fatalf("write broken skill: %v", err)
	}

	entries, warns := Discover([]string{root})
	if len(entries) != 2 {
		t.Fatalf("expected 2 valid discovered skills, got %d", len(entries))
	}

	foundHeading := false
	foundFrontmatter := false
	for _, e := range entries {
		switch e.Name {
		case "repo-context":
			foundHeading = true
			if e.Source != "heading" {
				t.Fatalf("expected repo-context source=heading, got %s", e.Source)
			}
			if e.Title != "Repo Context" {
				t.Fatalf("expected title Repo Context, got %s", e.Title)
			}
		case "go-review":
			foundFrontmatter = true
			if e.Source != "frontmatter.name" {
				t.Fatalf("expected go-review source=frontmatter.name, got %s", e.Source)
			}
			if e.Description != "Use when: review go changes" {
				t.Fatalf("unexpected description: %s", e.Description)
			}
			if e.Metadata["name"] != "go-review" {
				t.Fatalf("expected metadata name=go-review, got %#v", e.Metadata["name"])
			}
		}
	}
	if !foundHeading {
		t.Fatalf("expected discovered heading-based skill repo-context")
	}
	if !foundFrontmatter {
		t.Fatalf("expected discovered frontmatter-based skill go-review")
	}

	hasBrokenWarn := false
	for _, w := range warns {
		if strings.Contains(w, "parse skill failed") && strings.Contains(w, "broken-skill") {
			hasBrokenWarn = true
			break
		}
	}
	if !hasBrokenWarn {
		t.Fatalf("expected parse warning for broken-skill, got warns=%v", warns)
	}
}
