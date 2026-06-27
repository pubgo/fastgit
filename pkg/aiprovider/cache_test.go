package aiprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubProvider struct {
	calls int
	text  string
}

func (s *stubProvider) Name() string        { return "stub" }
func (s *stubProvider) Available() bool     { return true }
func (s *stubProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	s.calls++
	return CompleteResponse{Text: s.text, Provider: "stub"}, nil
}

func TestCachedProviderReusesResponse(t *testing.T) {
	stub := &stubProvider{text: "hello"}
	cached := WithCache(stub).(*cachedProvider)
	cached.dir = t.TempDir()

	req := CompleteRequest{System: "sys", User: "diff"}
	resp1, err := cached.Complete(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "hello", resp1.Text)

	resp2, err := cached.Complete(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "hello", resp2.Text)
	require.Equal(t, 1, stub.calls)
}
