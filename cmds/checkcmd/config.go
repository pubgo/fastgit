package checkcmd

import (
	"os"
	"path/filepath"
)

// Step defines one quality gate step.
type Step struct {
	Name        string
	Command     string
	FixCommand  string
	Fixable     bool
	Optional    bool
	Description string
}

// Config holds check pipeline configuration.
type Config struct {
	Steps []Step
}

// DefaultConfig returns the built-in pipeline for Go projects.
func DefaultConfig() Config {
	return Config{
		Steps: []Step{
			{
				Name:        "fmt",
				Command:     "gofmt -l .",
				FixCommand:  "gofmt -w .",
				Fixable:     true,
				Description: "Go source formatting",
			},
			{
				Name:        "vet",
				Command:     "go vet ./...",
				Description: "Go static analysis (go vet)",
			},
			{
				Name:        "test",
				Command:     "go test -short ./...",
				Description: "Unit tests (short mode)",
			},
			{
				Name:        "lint",
				Command:     "golangci-lint run ./...",
				Optional:    true,
				Description: "Lint (skipped if golangci-lint not in PATH)",
			},
			{
				Name:        "secrets",
				Command:     "",
				Optional:    true,
				Description: "Secret scan (configure gitleaks/trufflehog in .fastgit/check.yaml)",
			},
		},
	}
}

// LoadConfig loads check config; falls back to defaults until .fastgit/check.yaml is implemented.
func LoadConfig(repoRoot string) Config {
	_ = repoRoot
	configPath := filepath.Join(repoRoot, ".fastgit", "check.yaml")
	if _, err := os.Stat(configPath); err != nil {
		return DefaultConfig()
	}
	// TODO: parse .fastgit/check.yaml when schema is defined
	return DefaultConfig()
}
