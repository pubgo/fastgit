package acpui

import (
	"context"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"

	"github.com/pubgo/fastgit/pkg/acpclient"
)

func TestPermissionModeMapping(t *testing.T) {
	h, err := Permission("deny")
	require.NoError(t, err)
	require.Nil(t, h)

	h, err = Permission("auto")
	require.NoError(t, err)
	require.NotNil(t, h)

	h, err = Permission("")
	require.NoError(t, err)
	require.NotNil(t, h) // default ask, handler built without touching tty

	_, err = Permission("bogus")
	require.Error(t, err)
}

func TestPermissionAllowOncePrefersAllowAlwaysWhenNoOnce(t *testing.T) {
	resp, err := acpclient.PermissionAllowOnce(context.Background(), acp.RequestPermissionRequest{
		Options: []acp.PermissionOption{
			{OptionId: "always", Name: "Always", Kind: acp.PermissionOptionKindAllowAlways},
			{OptionId: "reject", Name: "Reject", Kind: acp.PermissionOptionKindRejectOnce},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Outcome.Selected)
	require.Equal(t, acp.PermissionOptionId("always"), resp.Outcome.Selected.OptionId)
}
