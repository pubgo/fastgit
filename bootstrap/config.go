package bootstrap

import (
	"context"
	"log/slog"
	"os"

	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/config"
	"github.com/pubgo/funk/v2/env"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/pathutil"
	"github.com/pubgo/funk/v2/running"
	"gopkg.in/yaml.v3"

	"github.com/pubgo/fastgit/cmds/fastcommitcmd"
	"github.com/pubgo/fastgit/configs"
	"github.com/pubgo/fastgit/utils"
)

type configProvider struct {
	Version      *configs.Version      `yaml:"version"`
	OpenaiConfig *utils.OpenaiConfig   `yaml:"openai"`
	CommitConfig *fastcommitcmd.Config `yaml:"commit"`
}

func initConfig() {
	slog.SetDefault(slog.New(log.NewSlog(log.GetLogger(""))))
	log.SetEnableChecker(func(ctx context.Context, lvl log.Level, name, message string, fields log.Fields) bool {
		if running.Debug.Value() {
			return true
		}

		if name == "dix" || name == "env" || fields["module"] == "env" {
			return false
		}
		return true
	})

	env.MustSet("LC_ALL", "C")
	env.LoadFiles(configs.GetLocalEnvPath()).Must()

	configPath := configs.GetConfigPath()
	envPath := configs.GetEnvPath()
	if pathutil.IsNotExist(configPath) {
		assert.Must(os.WriteFile(configPath, configs.GetDefaultConfig(), 0644))
		assert.Must(os.WriteFile(envPath, configs.GetEnvConfig(), 0644))
		return
	}

	type versionConfigProvider struct {
		Version *configs.Version `yaml:"version"`
	}
	var cfg versionConfigProvider
	config.LoadFromPath(&cfg, configPath)

	var defaultCfg versionConfigProvider
	defaultConfigData := configs.GetDefaultConfig()
	assert.Must(yaml.Unmarshal(defaultConfigData, &defaultCfg))
	if cfg.Version == nil || cfg.Version.Name == "" || defaultCfg.Version.Name != cfg.Version.Name {
		assert.Must(os.WriteFile(configPath, defaultConfigData, 0644))
		assert.Must(os.WriteFile(envPath, configs.GetEnvConfig(), 0644))
	}

	config.SetConfigPath(configPath)
}
