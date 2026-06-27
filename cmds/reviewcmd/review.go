package reviewcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/fastgit/pkg/aiprovider"
)

const reviewSystemPrompt = `You are a code reviewer. Review the git diff.

Output markdown with exactly these sections (use "-" bullets):
## Blockers
Critical bugs, security issues, or logic errors. Write "None" if none.

## Suggestions
Performance, readability, or maintainability improvements. Write "None" if none.

## Nits
Minor style or optional improvements. Write "None" if none.

## Test plan
Concrete verification steps for this change.

Be concise. Do not invent changes not present in the diff.`

// ReviewDiff runs AI review on a unified diff string.
func ReviewDiff(ctx context.Context, provider aiprovider.Provider, diff string, dryRun bool) (string, error) {
	diff = strings.TrimSpace(diff)
	if diff == "" {
		return "", fmt.Errorf("no diff to review")
	}
	if dryRun {
		return fmt.Sprintf("[dry-run] would review diff (%d bytes)", len(diff)), nil
	}
	if provider == nil || !provider.Available() {
		return ruleBasedReview(diff), nil
	}

	resp, err := provider.Complete(ctx, aiprovider.CompleteRequest{
		System: reviewSystemPrompt,
		User:   diff,
	})
	if err != nil {
		return ruleBasedReview(diff), err
	}
	text := strings.TrimSpace(resp.Text)
	if text == "" {
		return ruleBasedReview(diff), fmt.Errorf("empty review response")
	}
	return text, nil
}

// ReviewStaged runs AI review on a staged diff.
func ReviewStaged(ctx context.Context, provider aiprovider.Provider, diff string, dryRun bool) (string, error) {
	return ReviewDiff(ctx, provider, diff, dryRun)
}

func ruleBasedReview(diff string) string {
	lines := strings.Split(diff, "\n")
	fileCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			fileCount++
		}
	}
	var b strings.Builder
	b.WriteString("## Blockers\n\nNone (rule-based review; AI unavailable)\n\n")
	b.WriteString("## Suggestions\n\n")
	b.WriteString(fmt.Sprintf("- Review %d changed file(s) manually\n", fileCount))
	b.WriteString("- Run `fastgit check run --staged-only`\n\n")
	b.WriteString("## Nits\n\nNone\n\n")
	b.WriteString("## Test plan\n\n- [ ] Run tests for affected packages\n")
	return b.String()
}
