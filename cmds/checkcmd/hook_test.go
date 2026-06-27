package checkcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pubgo/redant"
	"github.com/stretchr/testify/require"
)

func TestManagedHookScripts(t *testing.T) {
	require.Len(t, managedHooks, 2)
	require.Equal(t, "pre-commit", managedHooks[0].name)
	require.Equal(t, "pre-push", managedHooks[1].name)
	require.Contains(t, managedHooks[0].script, "--staged-only")
	require.Contains(t, managedHooks[1].script, "check run")
}

func TestInstallHooks(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "init", repo)
	require.NoError(t, cmd.Run())

	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repo))
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	inv := &redant.Invocation{Stdout: os.Stdout}
	require.NoError(t, installHooks(inv, false))

	preCommit, err := os.ReadFile(filepath.Join(repo, ".git", "hooks", "pre-commit"))
	require.NoError(t, err)
	require.Contains(t, string(preCommit), hookMarker)

	prePush, err := os.ReadFile(filepath.Join(repo, ".git", "hooks", "pre-push"))
	require.NoError(t, err)
	require.Contains(t, string(prePush), hookMarker)
}
