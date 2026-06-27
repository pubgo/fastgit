package aiprovider

import (
	"context"
	"testing"

	"github.com/pubgo/fastgit/utils"
	"github.com/stretchr/testify/require"
)

func TestCommitMessageFromDiff(t *testing.T) {
	diff := `diff --git a/pkg/a.go b/pkg/a.go
index 111..222 100644
--- a/pkg/a.go
+++ b/pkg/a.go
`
	require.Equal(t, "chore: update pkg/a.go", CommitMessageFromDiff(diff))

	multi := diff + `diff --git a/pkg/b.go b/pkg/b.go
index 111..222 100644
--- a/pkg/b.go
+++ b/pkg/b.go
`
	require.Equal(t, "chore: update 2 files", CommitMessageFromDiff(multi))
}

func TestChainUsesFallbackWhenOpenAIUnavailable(t *testing.T) {
	chain := NewChain(NewOpenAI(nil), NewRuleFallback())
	require.True(t, chain.Available())

	resp, err := chain.Complete(context.Background(), CompleteRequest{
		System: "system",
		User:   "diff --git a/main.go b/main.go\n",
	})
	require.NoError(t, err)
	require.True(t, resp.Fallback)
	require.Equal(t, "rule-fallback", resp.Provider)
	require.Equal(t, "chore: update main.go", resp.Text)
}

func TestOpenAIAvailableRequiresAPIKey(t *testing.T) {
	p := NewOpenAI(&utils.OpenaiClient{Cfg: &utils.OpenaiConfig{ApiKey: ""}})
	require.False(t, p.Available())

	p = NewOpenAI(&utils.OpenaiClient{Cfg: &utils.OpenaiConfig{ApiKey: "sk-test"}})
	require.True(t, p.Available())
}
