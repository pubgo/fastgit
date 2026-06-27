package copilotperm

import (
	"context"
	"testing"
	"time"

	copilot "github.com/github/copilot-sdk/go"
	"github.com/stretchr/testify/require"
)

func TestBrokerResolveLatest(t *testing.T) {
	b := NewBroker()
	done := make(chan bool, 1)

	go func() {
		prompter := b.Prompter()
		ok, err := prompter(context.Background(), copilot.PermissionRequest{
			Kind: copilot.PermissionRequestKindRead,
		}, "read file")
		require.NoError(t, err)
		done <- ok
	}()

	deadline := time.After(2 * time.Second)
	for {
		pending := b.Pending()
		if len(pending) > 0 {
			require.NoError(t, b.ResolveLatest(true))
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for pending permission")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	require.True(t, <-done)
	require.Empty(t, b.Pending())
}
