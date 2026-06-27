package aiprovider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// CommitCandidate is one generated commit message option.
type CommitCandidate struct {
	Style   string
	Message string
}

const multiCandidateSystemPrompt = `Generate exactly 3 git commit message candidates for the provided diff.
Use present tense and conventional commit style where appropriate.

Return exactly 3 lines in this format:
SHORT: <max 40 chars, minimal>
MEDIUM: <max 72 chars, descriptive>
CONVENTIONAL: <type>(optional scope): <message>

If the change is breaking, append ! after the type in CONVENTIONAL (e.g. feat!: ...).`

var candidateLinePattern = regexp.MustCompile(`^(SHORT|MEDIUM|CONVENTIONAL):\s*(.+)$`)

// GenerateCommitCandidates asks the provider for 3 commit message options.
func GenerateCommitCandidates(ctx context.Context, provider Provider, diff string) ([]CommitCandidate, error) {
	if provider == nil || !provider.Available() {
		return ruleCommitCandidates(diff), nil
	}

	resp, err := provider.Complete(ctx, CompleteRequest{
		System: multiCandidateSystemPrompt,
		User:   diff,
	})
	if err != nil || strings.TrimSpace(resp.Text) == "" {
		return ruleCommitCandidates(diff), err
	}

	candidates := parseCommitCandidates(resp.Text)
	if len(candidates) == 0 {
		fallback := CommitMessageFromDiff(diff)
		return []CommitCandidate{{Style: "fallback", Message: fallback}}, nil
	}
	return candidates, nil
}

func parseCommitCandidates(text string) []CommitCandidate {
	var out []CommitCandidate
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		match := candidateLinePattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		out = append(out, CommitCandidate{
			Style:   strings.ToLower(match[1]),
			Message: strings.TrimSpace(match[2]),
		})
	}
	return out
}

func ruleCommitCandidates(diff string) []CommitCandidate {
	msg := CommitMessageFromDiff(diff)
	return []CommitCandidate{
		{Style: "short", Message: truncateRunes(msg, 40)},
		{Style: "medium", Message: truncateRunes(msg, 72)},
		{Style: "conventional", Message: msg},
	}
}

// DetectBreakingChange heuristically flags potentially breaking diffs.
func DetectBreakingChange(diff string) bool {
	lower := strings.ToLower(diff)
	tokens := []string{
		"breaking change",
		"!:",
		"remove ",
		"deleted ",
		"-export ",
		"rename ",
		"migration",
	}
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// BreakingChangeHint returns a user-facing hint for breaking changes.
func BreakingChangeHint(diff string) string {
	if !DetectBreakingChange(diff) {
		return ""
	}
	return "Possible breaking change detected — consider feat! or BREAKING CHANGE footer."
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

// FormatCandidateLabel renders a select option label.
func FormatCandidateLabel(c CommitCandidate) string {
	style := strings.TrimSpace(c.Style)
	if style == "" {
		style = "option"
	}
	return fmt.Sprintf("[%s] %s", strings.ToUpper(style), strings.TrimSpace(c.Message))
}
