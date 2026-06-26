package pullcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildEditorCommand(t *testing.T) {
	t.Run("editor with args", func(t *testing.T) {
		args := buildEditorCommand("code -w", "a.txt")
		require.Equal(t, []string{"code", "-w", "a.txt"}, args)
	})

	t.Run("editor without args", func(t *testing.T) {
		args := buildEditorCommand("vim", "a.txt")
		require.Equal(t, []string{"vim", "a.txt"}, args)
	})
}

func TestSplitRemoteRef(t *testing.T) {
	t.Run("standard origin branch", func(t *testing.T) {
		remote, branch := splitRemoteRef("origin/main")
		require.Equal(t, "origin", remote)
		require.Equal(t, "main", branch)
	})

	t.Run("nested remote branch", func(t *testing.T) {
		remote, branch := splitRemoteRef("upstream/feature/demo")
		require.Equal(t, "upstream", remote)
		require.Equal(t, "feature/demo", branch)
	})
}
