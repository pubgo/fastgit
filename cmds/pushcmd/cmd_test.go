package pushcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cmd := New()

	require.NotNil(t, cmd)
	require.Equal(t, "push", cmd.Use)
	require.Equal(t, "git push to remote origin", cmd.Short)
	require.NotNil(t, cmd.Handler)

	require.Len(t, cmd.Options, 3)
	require.Equal(t, "all", cmd.Options[0].Flag)
	require.Equal(t, "force", cmd.Options[1].Flag)
	require.Equal(t, "override-policy", cmd.Options[2].Flag)
}
