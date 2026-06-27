package aiprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/fastgit/utils"
	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider using an OpenAI-compatible API.
type OpenAIProvider struct {
	client *utils.OpenaiClient
}

// NewOpenAI wraps an existing OpenaiClient.
func NewOpenAI(client *utils.OpenaiClient) *OpenAIProvider {
	return &OpenAIProvider{client: client}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Available() bool {
	return p != nil &&
		p.client != nil &&
		p.client.Cfg != nil &&
		strings.TrimSpace(p.client.Cfg.ApiKey) != ""
}

func (p *OpenAIProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	if !p.Available() {
		return CompleteResponse{}, fmt.Errorf("openai provider unavailable: missing API key")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(p.client.Cfg.Model)
	}

	resp, err := p.client.Client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: req.System},
			{Role: openai.ChatMessageRoleUser, Content: req.User},
		},
	})
	if err != nil {
		return CompleteResponse{}, fmt.Errorf("openai completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return CompleteResponse{}, fmt.Errorf("openai completion: empty response")
	}

	return CompleteResponse{
		Text:     strings.TrimSpace(resp.Choices[0].Message.Content),
		Provider: p.Name(),
		Model:    model,
		Usage:    resp.Usage,
	}, nil
}
