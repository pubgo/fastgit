package aiprovider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCommitCandidates(t *testing.T) {
	text := `SHORT: quick fix
MEDIUM: fix parser edge case in auth module
CONVENTIONAL: fix(auth): handle empty token`
	candidates := parseCommitCandidates(text)
	require.Len(t, candidates, 3)
	require.Equal(t, "short", candidates[0].Style)
	require.Equal(t, "fix(auth): handle empty token", candidates[2].Message)
}

func TestDetectBreakingChange(t *testing.T) {
	require.True(t, DetectBreakingChange("feat!: remove legacy API"))
	require.False(t, DetectBreakingChange("chore: update docs"))
}

func TestRuleCommitCandidates(t *testing.T) {
	diff := "diff --git a/main.go b/main.go\n"
	candidates := ruleCommitCandidates(diff)
	require.Len(t, candidates, 3)
	require.Contains(t, candidates[2].Message, "main.go")
}
