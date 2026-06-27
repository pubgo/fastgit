package prcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/fastgit/cmds/reviewcmd"
	"github.com/pubgo/fastgit/pkg/aiprovider"
)

type draftEnhanceOptions struct {
	useAI      bool
	useReview  bool
	aiProvider string
	repoRoot   string
	dryRun     bool
}

func branchDiff(ctx context.Context, rc RepoContext) (string, error) {
	diff, err := gitOutput(ctx, rc.RepoRoot, "diff", rc.BaseRef+"..HEAD")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("no diff between %s and HEAD", rc.BaseRef)
	}
	return diff, nil
}

func enhanceDraft(ctx context.Context, draft Draft, rc RepoContext, opts draftEnhanceOptions) (Draft, error) {
	var reviewErr error
	if opts.useReview {
		diff, err := branchDiff(ctx, rc)
		if err != nil {
			reviewErr = fmt.Errorf("review: %w", err)
		} else {
			provider := aiprovider.ResolveProvider(opts.aiProvider, opts.repoRoot)
			report, err := reviewcmd.ReviewDiff(ctx, provider, diff, opts.dryRun)
			if err != nil && !opts.dryRun {
				reviewErr = fmt.Errorf("review: %w", err)
			}
			if strings.TrimSpace(report) != "" {
				draft.Body = ApplyReviewToBody(draft.Body, report)
			}
		}
	}

	if opts.useAI {
		provider := aiprovider.ResolveProvider(opts.aiProvider, opts.repoRoot)
		enhanced, ok, err := EnhanceDraft(ctx, provider, draft)
		if err != nil {
			if reviewErr != nil {
				return draft, reviewErr
			}
			return draft, fmt.Errorf("ai: %w", err)
		}
		if ok {
			draft = enhanced
		}
	}

	return draft, reviewErr
}

// ApplyReviewToBody merges a review report into the PR Test plan section.
func ApplyReviewToBody(body, review string) string {
	review = strings.TrimSpace(review)
	if review == "" {
		return body
	}

	summary := formatReviewSummary(review)
	testItems := reviewTestPlanItems(review)
	body = appendTestPlanItems(body, testItems)

	marker := "## Test plan\n\n"
	idx := strings.Index(body, marker)
	if idx < 0 {
		return body + "\n\n### Local code review\n\n" + summary
	}

	insertAt := idx + len(marker)
	rest := body[insertAt:]
	sectionEnd := len(body)
	if next := strings.Index(rest, "\n## "); next >= 0 {
		sectionEnd = insertAt + next
	}

	var b strings.Builder
	b.WriteString(body[:insertAt])
	b.WriteString(body[insertAt:sectionEnd])
	if !strings.HasSuffix(strings.TrimSpace(body[insertAt:sectionEnd]), "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("\n### Local code review\n\n")
	b.WriteString(summary)
	b.WriteString(body[sectionEnd:])
	return b.String()
}

func formatReviewSummary(review string) string {
	var b strings.Builder
	for _, section := range []struct {
		title string
		name  string
	}{
		{"Blockers", "Blockers"},
		{"Suggestions", "Suggestions"},
		{"Nits", "Nits"},
	} {
		content := extractReviewSection(review, section.name)
		if !sectionHasContent(content) {
			continue
		}
		b.WriteString("**")
		b.WriteString(section.title)
		b.WriteString("**\n\n")
		b.WriteString(strings.TrimSpace(content))
		b.WriteByte('\n')
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func reviewTestPlanItems(review string) []string {
	content := extractReviewSection(review, "Test plan")
	if !sectionHasContent(content) {
		return nil
	}
	var items []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "- [ ]")
		line = strings.TrimPrefix(line, "- [x]")
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "none") {
			continue
		}
		items = append(items, line)
	}
	return items
}

func appendTestPlanItems(body string, items []string) string {
	if len(items) == 0 {
		return body
	}
	marker := "## Test plan\n\n"
	idx := strings.Index(body, marker)
	if idx < 0 {
		return body
	}
	insertAt := idx + len(marker)
	rest := body[insertAt:]
	sectionEnd := len(body)
	if next := strings.Index(rest, "\n## "); next >= 0 {
		sectionEnd = insertAt + next
	}

	existing := body[insertAt:sectionEnd]
	var b strings.Builder
	b.WriteString(body[:insertAt])
	b.WriteString(existing)
	for _, item := range items {
		if strings.Contains(existing, item) {
			continue
		}
		if !strings.HasSuffix(existing, "\n") && len(existing) > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("- [ ] ")
		b.WriteString(item)
		b.WriteByte('\n')
	}
	b.WriteString(body[sectionEnd:])
	return b.String()
}

func extractReviewSection(review, name string) string {
	header := "## " + name
	idx := strings.Index(review, header)
	if idx < 0 {
		return ""
	}
	start := idx + len(header)
	rest := review[start:]
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	end := len(rest)
	for _, other := range []string{"## Blockers", "## Suggestions", "## Nits", "## Test plan"} {
		if other == header {
			continue
		}
		if pos := strings.Index(rest, "\n"+other); pos >= 0 && pos < end {
			end = pos
		}
	}
	return strings.TrimSpace(rest[:end])
}

func sectionHasContent(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	lower := strings.ToLower(content)
	return lower != "none" && !strings.HasPrefix(lower, "none (")
}
