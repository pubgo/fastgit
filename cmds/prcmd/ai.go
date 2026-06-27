package prcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/fastgit/pkg/aiprovider"
)

const prEnhanceSystemPrompt = `You improve GitHub pull request title and description.
Keep markdown sections: Summary, Changed files, Risk, Test plan, Rollback.
Be concise and actionable. Do not invent changes not present in the draft.

Reply in this exact format:
TITLE: <single line title, max 72 chars>
BODY:
<full markdown body>`

// EnhanceDraft optionally refines a rule-based PR draft using AI.
func EnhanceDraft(ctx context.Context, provider aiprovider.Provider, draft Draft) (Draft, bool, error) {
	if provider == nil || !provider.Available() {
		return draft, false, nil
	}

	userPrompt := fmt.Sprintf("Branch: %s\nBase: %s\n\nCurrent draft title:\n%s\n\nCurrent draft body:\n%s",
		draft.Head, draft.Base, draft.Title, draft.Body)

	resp, err := provider.Complete(ctx, aiprovider.CompleteRequest{
		System: prEnhanceSystemPrompt,
		User:   userPrompt,
	})
	if err != nil {
		return draft, false, err
	}

	title, body, ok := parseEnhancedPR(resp.Text, draft)
	if !ok {
		return draft, false, fmt.Errorf("unable to parse AI PR response")
	}
	draft.Title = title
	draft.Body = body
	return draft, true, nil
}

func parseEnhancedPR(text string, fallback Draft) (title, body string, ok bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}

	lines := strings.Split(text, "\n")
	titleIdx, bodyStart := -1, -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "TITLE:") {
			titleIdx = i
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "BODY:" || strings.HasPrefix(trimmed, "BODY:") {
			bodyStart = i + 1
			if strings.HasPrefix(trimmed, "BODY:") && len(trimmed) > len("BODY:") {
				rest := strings.TrimSpace(trimmed[len("BODY:"):])
				if rest != "" {
					bodyStart = i
					lines[i] = rest
				}
			}
			break
		}
	}

	if titleIdx >= 0 {
		title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[titleIdx]), "TITLE:"))
	}
	if bodyStart >= 0 && bodyStart < len(lines) {
		body = strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	}

	if title == "" {
		title = fallback.Title
	}
	if body == "" {
		body = fallback.Body
	}

	ok = titleIdx >= 0 && bodyStart > 0 && strings.TrimSpace(body) != ""
	return title, body, ok
}
