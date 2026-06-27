package copilotperm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobalBrokerPrompter(t *testing.T) {
	b := NewBroker()
	SetGlobalBroker(b)
	t.Cleanup(func() { SetGlobalBroker(nil) })

	p := PrompterForMode(ModeAsk, nil)
	require.NotNil(t, p)

	p = PrompterForMode(ModeAllow, nil)
	require.Nil(t, p)
}
