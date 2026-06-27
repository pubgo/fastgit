package aiprovider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopilotCLIExists(t *testing.T) {
	require.False(t, CopilotCLIExists("/nonexistent/copilot"))
}

func TestComposePrompt(t *testing.T) {
	require.Equal(t, "user only", composePrompt(CompleteRequest{User: "user only"}))
	require.Equal(t, "system\n\nuser", composePrompt(CompleteRequest{System: "system", User: "user"}))
}

func TestResolveProviderNames(t *testing.T) {
	require.Equal(t, "copilot", ResolveProvider("copilot", ".").Name())
	require.Equal(t, "chain", ResolveProvider("openai", ".").Name())
	require.Equal(t, "chain", ResolveProvider("auto", ".").Name())
}
