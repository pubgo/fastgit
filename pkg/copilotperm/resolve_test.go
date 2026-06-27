package copilotperm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveModePriority(t *testing.T) {
	t.Setenv(permissionModeEnv, "deny")

	mode, err := ResolveMode("allow", DefaultMode)
	require.NoError(t, err)
	require.Equal(t, ModeAllow, mode)

	mode, err = ResolveMode("", DefaultMode)
	require.NoError(t, err)
	require.Equal(t, ModeDeny, mode)

	t.Setenv(permissionModeEnv, "")
	mode, err = ResolveMode("", ModeDeny)
	require.NoError(t, err)
	require.Equal(t, ModeDeny, mode)
}

func TestInjectPermissionMode(t *testing.T) {
	args := InjectPermissionMode([]string{"chat", "--prompt", "hi"}, ModeAsk)
	require.Equal(t, []string{"--permission-mode", "ask", "chat", "--prompt", "hi"}, args)

	existing := []string{"chat", "--permission-mode", "deny", "--prompt", "hi"}
	require.Equal(t, existing, InjectPermissionMode(existing, ModeAsk))
	require.True(t, HasPermissionModeFlag(existing))
}
