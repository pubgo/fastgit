package configs

import (
	_ "embed"
	"path"
	"strings"
	"sync"

	"github.com/adrg/xdg"
	"github.com/bitfield/script"
	"github.com/pubgo/funk/v2/assert"
)

type Version struct {
	Name string `yaml:"name"`
}

//go:embed default.yaml
var defaultConfig []byte

//go:embed env.yaml
var envConfig []byte

var GetConfigPath = sync.OnceValue(func() string {
	return assert.Exit1(xdg.ConfigFile("fastgit/config.yaml"))
})

var GetRepoPath = sync.OnceValue(func() string {
	repoPath := assert.Exit1(script.Exec("git rev-parse --show-toplevel").String())
	return strings.TrimSpace(repoPath)
})

var GetEnvPath = sync.OnceValue(func() string {
	return path.Join(path.Dir(GetConfigPath()), "env.yaml")
})

var GetLocalEnvPath = sync.OnceValue(func() string {
	return path.Join(GetRepoPath(), ".git", "fastgit.env")
})

func GetDefaultConfig() []byte { return defaultConfig }

func GetEnvConfig() []byte { return envConfig }
