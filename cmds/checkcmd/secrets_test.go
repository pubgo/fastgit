package checkcmd

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSecretScanCommand(t *testing.T) {
	cmd, tool := resolveSecretScanCommand(false)
	if _, err := exec.LookPath("gitleaks"); err == nil {
		require.Equal(t, "gitleaks", tool)
		require.Contains(t, cmd, "gitleaks detect")
		return
	}
	if _, err := exec.LookPath("trufflehog"); err == nil {
		require.Equal(t, "trufflehog", tool)
		require.Contains(t, cmd, "trufflehog")
		return
	}
	require.Empty(t, cmd)
	require.Empty(t, tool)
}

func TestResolveSecretScanCommandStaged(t *testing.T) {
	cmd, tool := resolveSecretScanCommand(true)
	if _, err := exec.LookPath("gitleaks"); err == nil {
		require.Equal(t, "gitleaks", tool)
		require.Contains(t, cmd, "protect --staged")
		return
	}
	if _, err := exec.LookPath("trufflehog"); err == nil {
		require.Equal(t, "trufflehog", tool)
		require.Contains(t, cmd, "--staged")
		return
	}
	require.Empty(t, cmd)
}
