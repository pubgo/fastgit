package checkcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	root := New()
	require.NotNil(t, root)
	require.Equal(t, "check", root.Use)
	require.Len(t, root.Children, 3)

	run := root.Children[0]
	require.Equal(t, "run", run.Use)
	require.NotNil(t, run.Handler)
	require.Len(t, run.Options, 3)
	require.Equal(t, "staged-only", run.Options[0].Flag)
	require.Equal(t, "fix", run.Options[1].Flag)
	require.Equal(t, "dry-run", run.Options[2].Flag)

	config := root.Children[1]
	require.Equal(t, "config", config.Use)
	require.NotNil(t, config.Handler)

	hook := root.Children[2]
	require.Equal(t, "hook", hook.Use)
	require.Len(t, hook.Children, 2)
	require.Equal(t, "install", hook.Children[0].Use)
	require.Equal(t, "uninstall", hook.Children[1].Use)
}
