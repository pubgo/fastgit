package initcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pubgo/fastgit/configs"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/config"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/pathutil"
	"github.com/pubgo/funk/v2/recovery"
	"github.com/pubgo/funk/v2/strutil"
	"github.com/pubgo/redant"
)

func New() *redant.Command {
	return &redant.Command{
		Use:   "init",
		Short: "initialize config/env/local env files",
		Handler: func(ctx context.Context, i *redant.Invocation) error {
			defer recovery.Exit()

			cfgPath := configs.GetConfigPath()
			envPath := configs.GetEnvPath()
			localPath := configs.GetLocalEnvPath()

			if pathutil.IsNotExist(cfgPath) {
				assert.Must(os.WriteFile(cfgPath, configs.GetDefaultConfig(), 0644))
				log.Info().Msgf("config created: %s", cfgPath)
			} else {
				log.Info().Msgf("config exists: %s", cfgPath)
			}

			if pathutil.IsNotExist(envPath) {
				assert.Must(os.WriteFile(envPath, configs.GetEnvConfig(), 0644))
				log.Info().Msgf("env template created: %s", envPath)
			} else {
				log.Info().Msgf("env template exists: %s", envPath)
			}

			if pathutil.IsNotExist(localPath) {
				file := assert.Exit1(os.Create(localPath))
				defer file.Close()
				for name, cfg := range config.LoadEnvMap(cfgPath) {
					envData := strutil.FirstNotEmpty(cfg.Value, cfg.Default, "")
					fmt.Fprintf(file, "%s=%q\n", name, envData)
				}
				log.Info().Msgf("local env created: %s", localPath)
			} else {
				log.Info().Msgf("local env exists: %s", localPath)
			}

			return nil
		},
	}
}
