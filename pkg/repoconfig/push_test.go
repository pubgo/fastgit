package repoconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePushBlocksProtectedBranch(t *testing.T) {
	bundle := Bundle{Policy: Policy{ProtectedBranches: []string{"main", "master"}}}
	require.Error(t, bundle.ValidatePush("main", false))
	require.NoError(t, bundle.ValidatePush("feature/x", false))
	require.NoError(t, bundle.ValidatePush("main", true))
}
