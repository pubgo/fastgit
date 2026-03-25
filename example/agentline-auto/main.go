package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pubgo/redant"
	agentlineapp "github.com/pubgo/redant/cmds/agentlineapp"
	agentlinemodule "github.com/pubgo/redant/pkg/agentline"
)

func main() {
	var message string

	commitCmd := &redant.Command{
		Use:      "commit",
		Short:    "Dual-mode command: normal CLI + slash command in agentline.",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "message", Shorthand: "m", Description: "Commit message.", Value: redant.StringOf(&message), Default: "chore: update"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_, err := fmt.Fprintf(inv.Stdout, "[commit] message=%q args=%v\n", message, inv.Args)
			return err
		},
	}

	statusCmd := &redant.Command{
		Use:   "status",
		Short: "Normal command: runs directly without agentline redirect.",
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_, err := fmt.Fprintln(inv.Stdout, "[status] working tree clean")
			return err
		},
	}

	rootCmd := &redant.Command{
		Use:   "agentline-auto",
		Short: "Example for agentline auto-dispatch via command metadata.",
		Children: []*redant.Command{
			commitCmd,
			statusCmd,
		},
	}

	rootCmd.Handler = func(ctx context.Context, inv *redant.Invocation) error {
		return agentlineapp.Run(ctx, rootCmd, &agentlineapp.RuntimeOptions{
			Prompt: "agent> ",
			Stdin:  inv.Stdin,
			Stdout: inv.Stdout,
		})
	}

	if err := rootCmd.Invoke().WithOS().Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
