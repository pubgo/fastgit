package gitconflict

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModuleName(t *testing.T) {
	require.Equal(t, "cmds/fastcommitcmd", moduleName("cmds/fastcommitcmd/ai.go"))
	require.Equal(t, "docs", moduleName("docs/features.md"))
	require.Equal(t, "(root)", moduleName("README.md"))
}

func TestSuggestReason(t *testing.T) {
	require.Contains(t, suggestReason("pkg/a.go"), "Code conflict")
	require.Contains(t, suggestReason("go.mod"), "Dependency")
}

func TestRenderSummary(t *testing.T) {
	files := []File{
		{Path: "a.go", Module: "(root)", Reason: "Code conflict"},
		{Path: "docs/x.md", Module: "docs", Reason: "Documentation conflict"},
	}
	groups := map[string][]string{
		"(root)": {"a.go"},
		"docs":   {"docs/x.md"},
	}
	summary := renderSummary(files, groups)
	require.Contains(t, summary, "2 conflicted file")
	require.Contains(t, summary, "a.go")
}
