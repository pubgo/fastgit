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
	"github.com/pubgo/fastcommit/cmds/chglogcmd"
	"github.com/pubgo/fastcommit/cmds/configcmd"
	"github.com/pubgo/fastcommit/cmds/devcmd"
	"github.com/pubgo/fastcommit/cmds/fastcommitcmd"
	"github.com/pubgo/fastcommit/cmds/historycmd"
	"github.com/pubgo/fastcommit/cmds/pullcmd"
	"github.com/pubgo/fastcommit/cmds/tagcmd"
	"github.com/pubgo/fastcommit/cmds/upgradecmd"
	"github.com/pubgo/fastcommit/cmds/versioncmd"
	"github.com/pubgo/fastcommit/utils"
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
		upgradecmd.New(),
		tagcmd.New(),
		historycmd.New(),
		fastcommitcmd.New(),
		configcmd.New(),
		pullcmd.New(),
		chglogcmd.NewCommand(),
		devcmd.New(),
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
		Use:      "fastcommit",
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
