package bootstrap

import (
	"context"
	"fmt"
	"os"

	_ "github.com/adrg/xdg"
	_ "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/pubgo/dix/v2"
	"github.com/pubgo/dix/v2/dixcontext"
	"github.com/pubgo/fastgit/cmds/chglogcmd"
	"github.com/pubgo/fastgit/cmds/configcmd"
	"github.com/pubgo/fastgit/cmds/fastcommitcmd"
	"github.com/pubgo/fastgit/cmds/historycmd"
	"github.com/pubgo/fastgit/cmds/initcmd"
	"github.com/pubgo/fastgit/cmds/pullcmd"
	"github.com/pubgo/fastgit/cmds/tagcmd"
	"github.com/pubgo/fastgit/cmds/upgradecmd"
	"github.com/pubgo/fastgit/cmds/versioncmd"
	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/config"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/recovery"
	"github.com/pubgo/redant"
	_ "github.com/sashabaranov/go-openai"
)

func Main() {
	run(
		versioncmd.New(),
		initcmd.New(),
		upgradecmd.New(),
		tagcmd.New(),
		historycmd.New(),
		fastcommitcmd.New(),
		configcmd.New(),
		pullcmd.New(),
		chglogcmd.NewCommand(),
	)
}

func run(cmds ...*redant.Command) {
	defer recovery.Exit(func(err error) error {
		if errors.Is(err, context.Canceled) {
			return nil
		}

		if err.Error() == "signal: interrupt" {
			return nil
		}

		log.Err(err).Msg("failed to run command")
		return nil
	})

	app := &redant.Command{
		Use:      "fastgit",
		Short:    "Intelligent generation of git commit message",
		Children: cmds,
		Middleware: func(next redant.HandlerFunc) redant.HandlerFunc {
			return func(ctx context.Context, i *redant.Invocation) error {
				if utils.IsHelp() {
					return redant.DefaultHelpFn()(ctx, i)
				}

				if !term.IsTerminal(os.Stdin.Fd()) {
					return fmt.Errorf("stdin is not terminal")
				}

				initConfig()
				di := dix.New(dix.WithValuesNull())
				di.Provide(config.Load[configProvider])
				di.Provide(utils.NewOpenaiClient)
				return next(dixcontext.Create(ctx, di), i)
			}
		},
	}

	assert.Must(app.Run(utils.Context()))
}
