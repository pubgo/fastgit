package checkcmd

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type stepYAML struct {
	Name        string `yaml:"name"`
	Command     string `yaml:"command"`
	FixCommand  string `yaml:"fix_command"`
	Fixable     bool   `yaml:"fixable"`
	Optional    bool   `yaml:"optional"`
	Description string `yaml:"description"`
}

type checkYAML struct {
	Steps []stepYAML `yaml:"steps"`
}

// Config holds check pipeline configuration.
type Config struct {
	Steps []Step
}

// Step defines one quality gate step.
type Step struct {
	Name        string
	Command     string
	FixCommand  string
	Fixable     bool
	Optional    bool
	Description string
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
				Description: "Secret scan (configure in .fastgit/check.yaml)",
			},
		},
	}
}

const defaultCheckYAML = `steps:
  - name: fmt
    command: gofmt -l .
    fix_command: gofmt -w .
    fixable: true
    description: Go source formatting
  - name: vet
    command: go vet ./...
    description: Go static analysis (go vet)
  - name: test
    command: go test -short ./...
    description: Unit tests (short mode)
  - name: lint
    command: golangci-lint run ./...
    optional: true
    description: Lint (skipped if golangci-lint not in PATH)
  - name: secrets
    command: ""
    optional: true
    description: Secret scan (gitleaks or trufflehog in PATH)
`

// InitConfigTemplate writes `.fastgit/check.yaml` when missing.
func InitConfigTemplate(repoRoot string) (string, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return "", nil
	}
	dir := filepath.Join(repoRoot, ".fastgit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "check.yaml")
	if _, err := os.Stat(path); err == nil {
		return "", nil
	}
	if err := os.WriteFile(path, []byte(defaultCheckYAML), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadConfig loads `.fastgit/check.yaml` or returns defaults.
func LoadConfig(repoRoot string) Config {
	cfg := DefaultConfig()
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return cfg
	}
	configPath := filepath.Join(repoRoot, ".fastgit", "check.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return cfg
	}

	var file checkYAML
	if err := yaml.Unmarshal(data, &file); err != nil {
		return cfg
	}
	if len(file.Steps) == 0 {
		return cfg
	}

	steps := make([]Step, 0, len(file.Steps))
	for _, item := range file.Steps {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		steps = append(steps, Step{
			Name:        name,
			Command:     strings.TrimSpace(item.Command),
			FixCommand:  strings.TrimSpace(item.FixCommand),
			Fixable:     item.Fixable,
			Optional:    item.Optional,
			Description: strings.TrimSpace(item.Description),
		})
	}
	if len(steps) == 0 {
		return cfg
	}
	return Config{Steps: steps}
}
