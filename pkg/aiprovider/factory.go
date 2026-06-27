package aiprovider

import "github.com/pubgo/fastgit/utils"

// Default builds the standard provider chain: OpenAI-compatible API, then rule fallback.
func Default(client *utils.OpenaiClient) Provider {
	return NewChain(
		NewOpenAI(client),
		NewRuleFallback(),
	)
}
