package reviewcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleBasedReview(t *testing.T) {
	diff := "diff --git a/main.go b/main.go\n+++ b/main.go\n"
	report := ruleBasedReview(diff)
	require.Contains(t, report, "## Blockers")
	require.Contains(t, report, "## Test plan")
	require.Contains(t, report, "1 changed file")
}
