package aiprovider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pubgo/fastgit/configs"
	"github.com/pubgo/fastgit/utils"
	"gopkg.in/yaml.v3"
)

type openAIConfigFile struct {
	Openai *utils.OpenaiConfig `yaml:"openai"`
}

// OpenAIProviderFromConfig loads OpenAI settings from the fastgit config file and env.
func OpenAIProviderFromConfig() *OpenAIProvider {
	cfg := &utils.OpenaiConfig{
		ApiKey:  strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		BaseURL: strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")),
		Model:   strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
	}

	configPath := configs.GetConfigPath()
	if data, err := os.ReadFile(configPath); err == nil {
		var file openAIConfigFile
		if err := yaml.Unmarshal(data, &file); err == nil && file.Openai != nil {
			merged := mergeOpenAIConfig(cfg, file.Openai)
			cfg = &merged
		}
	}

	return NewOpenAI(utils.NewOpenaiClient(cfg))
}

func mergeOpenAIConfig(base, from *utils.OpenaiConfig) utils.OpenaiConfig {
	out := utils.OpenaiConfig{}
	if base != nil {
		out = *base
	}
	if from == nil {
		return out
	}
	if strings.TrimSpace(from.ApiKey) != "" {
		out.ApiKey = from.ApiKey
	}
	if strings.TrimSpace(from.BaseURL) != "" {
		out.BaseURL = from.BaseURL
	}
	if strings.TrimSpace(from.Model) != "" {
		out.Model = from.Model
	}
	return out
}

// ResolveProvider picks a provider chain by name: auto|openai|copilot.
func ResolveProvider(name, workingDir string) Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "openai":
		return NewChain(OpenAIProviderFromConfig(), NewRuleFallback())
	case "copilot":
		return NewCopilot(DefaultCopilotConfig(workingDir))
	default:
		return NewChain(
			OpenAIProviderFromConfig(),
			NewCopilot(DefaultCopilotConfig(workingDir)),
			NewRuleFallback(),
		)
	}
}

// EnhanceText runs a completion and returns trimmed text, or the original on failure.
func EnhanceText(ctx context.Context, provider Provider, system, user, fallback string) (string, bool, error) {
	if provider == nil || !provider.Available() {
		return fallback, false, nil
	}
	resp, err := provider.Complete(ctx, CompleteRequest{System: system, User: user})
	if err != nil || strings.TrimSpace(resp.Text) == "" {
		return fallback, false, err
	}
	return strings.TrimSpace(resp.Text), !resp.Fallback, nil
}

// MustEnhanceText returns enhanced text or fallback without error.
func MustEnhanceText(ctx context.Context, provider Provider, system, user, fallback string) string {
	text, _, _ := EnhanceText(ctx, provider, system, user, fallback)
	return text
}

// FormatError wraps provider errors for CLI output.
func FormatError(providerName string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s provider: %w", providerName, err)
}
