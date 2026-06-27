package aiprovider

import (
	"os"
	"strings"

	"github.com/pubgo/fastgit/utils"
)

// Default builds the standard provider chain: OpenAI-compatible API, then rule fallback.
func Default(client *utils.OpenaiClient) Provider {
	chain := NewChain(
		NewOpenAI(client),
		NewRuleFallback(),
	)
	if cacheEnabled() {
		return WithCache(chain)
	}
	return chain
}

func cacheEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("FASTGIT_AI_CACHE")))
	return v == "1" || v == "true" || v == "yes"
}
