package checkcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoadConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".fastgit")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	path := filepath.Join(configDir, "check.yaml")
	raw, err := yaml.Marshal(checkYAML{
		Steps: []stepYAML{
			{Name: "fmt", Command: "gofmt -l .", Fixable: true},
			{Name: "custom", Command: "echo ok", Optional: true},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	cfg := LoadConfig(dir)
	require.Len(t, cfg.Steps, 2)
	require.Equal(t, "custom", cfg.Steps[1].Name)
}

func TestStagedPackagePatterns(t *testing.T) {
	patterns := stagedPackagePatterns([]string{"pkg/aiprovider/openai.go", "cmds/prcmd/cmd.go"})
	require.Contains(t, patterns, "./pkg/aiprovider/...")
	require.Contains(t, patterns, "./cmds/prcmd/...")
}

func TestStagedVetCommand(t *testing.T) {
	cmd := stagedVetCommand([]string{"pkg/foo/a.go"})
	require.Equal(t, "go vet ./pkg/foo/...", cmd)
}
