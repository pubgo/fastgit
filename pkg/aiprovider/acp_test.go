package aiprovider

import (
	"context"
	"testing"

	"github.com/pubgo/fastgit/pkg/acpconfig"
	"github.com/stretchr/testify/require"
)

func TestACPProviderName(t *testing.T) {
	require.Equal(t, "acp", NewACP("").Name())
}

func TestACPProviderUnavailableWhenAgentMissing(t *testing.T) {
	t.Setenv(acpconfig.EnvAgentVar, "ghost-agent-not-exist")
	require.False(t, NewACP("").Available())

	_, err := NewACP("").Complete(context.Background(), CompleteRequest{User: "hi"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ghost-agent-not-exist")
}

func TestDefaultProviderNameEnvOverride(t *testing.T) {
	t.Setenv("FASTGIT_AI_PROVIDER", "acp")
	require.Equal(t, "acp", defaultProviderName())
}

func TestResolveProviderACPChain(t *testing.T) {
	t.Setenv("FASTGIT_AI_PROVIDER", "")
	p := ResolveProvider("acp", t.TempDir())
	require.NotNil(t, p)
	require.IsType(t, (*Chain)(nil), p)
}
