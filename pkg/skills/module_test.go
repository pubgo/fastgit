package skills

import "testing"

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
