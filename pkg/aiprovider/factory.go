package aiprovider

import (
	"os"
	"strings"

	"github.com/pubgo/fastgit/utils"
)

// Default builds the provider used by the commit flow, selected by config
// (ai.provider) or FASTGIT_AI_PROVIDER: acp|openai|copilot|auto.
// auto prefers the external ACP agent, then OpenAI, copilot and rule fallback.
func Default(client *utils.OpenaiClient) Provider {
	var provider Provider
	switch strings.ToLower(strings.TrimSpace(defaultProviderName())) {
	case "acp":
		provider = NewChain(NewACP(""), NewRuleFallback())
	case "openai":
		provider = NewChain(NewOpenAI(client), NewRuleFallback())
	case "copilot":
		provider = NewCopilot(DefaultCopilotConfig(""))
	default:
		provider = NewChain(
			NewACP(""),
			NewOpenAI(client),
			NewCopilot(DefaultCopilotConfig("")),
			NewRuleFallback(),
		)
	}
	if cacheEnabled() {
		return WithCache(provider)
	}
	return provider
}

func cacheEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("FASTGIT_AI_CACHE")))
	return v == "1" || v == "true" || v == "yes"
}
