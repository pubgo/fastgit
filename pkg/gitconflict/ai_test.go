package gitconflict

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractConflictRegions(t *testing.T) {
	content := "ok\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> branch\n"
	regions := extractConflictRegions(content)
	require.Contains(t, regions, "<<<<<<< HEAD")
	require.Contains(t, regions, "theirs")
}

func TestParseAIReasons(t *testing.T) {
	text := "FILE: pkg/a.go\nREASON: Both sides changed imports\n"
	reasons := parseAIReasons(text)
	require.Equal(t, "Both sides changed imports", reasons["pkg/a.go"])
}

func TestApplyAIReasons(t *testing.T) {
	snap := Snapshot{
		Files: []File{{Path: "a.go", Module: "(root)", Reason: "old"}},
		Groups: map[string][]string{
			"(root)": {"a.go"},
		},
	}
	applyAIReasons(&snap, "FILE: a.go\nREASON: signature mismatch\n")
	require.Contains(t, snap.Files[0].Reason, "signature mismatch")
	require.Contains(t, snap.Summary, "signature mismatch")
}
