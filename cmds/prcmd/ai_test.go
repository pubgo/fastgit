package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEnhancedPR(t *testing.T) {
	fallback := Draft{Title: "fallback title", Body: "fallback body"}
	text := `TITLE: feat: add pr ai enhance
BODY:
## Summary

- improved PR text

## Test plan

- [ ] run tests
`
	title, body, ok := parseEnhancedPR(text, fallback)
	require.True(t, ok)
	require.Equal(t, "feat: add pr ai enhance", title)
	require.Contains(t, body, "## Summary")
	require.Contains(t, body, "run tests")
}

func TestParseEnhancedPRKeepsFallbackWhenMissingSections(t *testing.T) {
	fallback := Draft{Title: "keep me", Body: "## Summary\n\noriginal"}
	title, body, ok := parseEnhancedPR("not valid", fallback)
	require.False(t, ok)
	require.Equal(t, "keep me", title)
	require.Equal(t, "## Summary\n\noriginal", body)
}
