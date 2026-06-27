package aiprovider

import (
	"context"
	"fmt"
	"strings"
)

// Chain tries providers in order until one succeeds.
type Chain struct {
	providers []Provider
}

// NewChain returns a provider chain.
func NewChain(providers ...Provider) *Chain {
	return &Chain{providers: providers}
}

func (c *Chain) Name() string { return "chain" }

func (c *Chain) Available() bool {
	if c == nil {
		return false
	}
	for _, p := range c.providers {
		if p != nil && p.Available() {
			return true
		}
	}
	return len(c.providers) > 0
}

func (c *Chain) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	if c == nil || len(c.providers) == 0 {
		return CompleteResponse{}, fmt.Errorf("no AI providers configured")
	}

	var lastErr error
	for _, provider := range c.providers {
		if provider == nil {
			continue
		}
		if !provider.Available() {
			continue
		}
		resp, err := provider.Complete(ctx, req)
		if err == nil && strings.TrimSpace(resp.Text) != "" {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		}
	}

	if lastErr != nil {
		return CompleteResponse{}, fmt.Errorf("all AI providers failed: %w", lastErr)
	}
	return CompleteResponse{}, fmt.Errorf("all AI providers failed")
}
