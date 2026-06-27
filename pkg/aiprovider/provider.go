package aiprovider

import "context"

// Provider is the unified AI completion interface for fastgit commands.
type Provider interface {
	Name() string
	Available() bool
	Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
}

// CompleteRequest is a chat-style completion input.
type CompleteRequest struct {
	System string
	User   string
	Model  string
}

// CompleteResponse is a normalized completion output.
type CompleteResponse struct {
	Text     string
	Provider string
	Model    string
	Usage    any
	Fallback bool
}
