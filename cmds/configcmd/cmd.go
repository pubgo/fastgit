package configcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/a8m/envsubst"
	"github.com/joho/godotenv"
	"github.com/pubgo/fastcommit/configs"
	"github.com/pubgo/fastcommit/utils"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/config"
	"github.com/pubgo/funk/v2/env"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/pathutil"
	"github.com/pubgo/funk/v2/pretty"
	"github.com/pubgo/funk/v2/recovery"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/funk/v2/strutil"
	"github.com/pubgo/redant"
	"github.com/samber/lo"
)

func New() *redant.Command {
	return &redant.Command{
		Use:   "config",
		Short: "config management",
		Children: []*redant.Command{
			{
				Use:   "edit",
				Short: "edit config, env or local env file, args: [config|env|local], default:config",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					command := i.Command
					args := command.Args
					if len(args) == 0 {
						utils.Edit(configs.GetConfigPath())
						return nil
					}

					switch args[0].Value.String() {
					case "config":
						utils.Edit(configs.GetConfigPath())
					case "env":
						utils.Edit(configs.GetEnvPath())
					case "local":
						if pathutil.IsNotExist(configs.GetLocalEnvPath()) {
							file := assert.Exit1(os.Create(configs.GetLocalEnvPath()))
							defer file.Close()
							for name, cfg := range config.LoadEnvMap(configs.GetConfigPath()) {
								envData := strutil.FirstNotEmpty(cfg.Value, cfg.Default, "")
								fmt.Fprintln(file, fmt.Sprintf(`%s=%q`, name, envData))
							}
						}
						utils.Edit(configs.GetLocalEnvPath())
					}

					return nil
				},
			},

			{
				Use:   "show",
				Short: "show config, env or local env file, args: [config|env|local], default:config",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					defer recovery.Exit()

					command := i.Command
					args := command.Args
					if len(args) == 0 || args[0].Value.String() == "config" {
						cfgPath := configs.GetConfigPath()
						log.Info().Msgf("config path: %s", cfgPath)

						cfgData := assert.Must1(os.ReadFile(cfgPath))
						cfgData = assert.Must1(envsubst.Bytes(cfgData))

						log.Info().Msgf("config data: \n%s", cfgData)
						return nil
					}

					switch args[0].Value.String() {
					case "env":
						log.Info().Msgf("env path: %s", configs.GetEnvPath())
						env.LoadFiles(configs.GetLocalEnvPath())
						envMap := config.LoadEnvMap(configs.GetConfigPath())
						for name, cfg := range envMap {
							envData := env.Get(name)
							if envData != "" {
								cfg.Value = envData
							}
						}

						pretty.Println(lo.Values(envMap))
					case "local":
						log.Info().Msgf("local env path: %s", configs.GetLocalEnvPath())
						data := result.Wrap(os.ReadFile(configs.GetLocalEnvPath())).Unwrap()
						dataMap := result.Wrap(godotenv.UnmarshalBytes(data)).Unwrap()
						pretty.Println(dataMap)
					}

					return nil
				},
			},
		},
	}
}
