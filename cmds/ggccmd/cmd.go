package ggccmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/redant"
)

func New() *redant.Command {
	registry := NewRegistry()
	store := NewStateStore()

	return &redant.Command{
		Use:   "ggc",
		Short: "Unified Git command surface inspired by ggc",
		Children: []*redant.Command{
			{
				Use:   "list",
				Short: "List supported unified commands",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					state, err := store.Load()
					if err != nil {
						return err
					}

					for _, entry := range buildInteractiveEntries(registry, state) {
						fmt.Printf("- %-28s %s\n", entry.Usage, entry.Description)
					}
					return nil
				},
			},
			{
				Use:   "interactive",
				Short: "Run interactive fuzzy command picker",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					return runInteractiveFlow(ctx, registry, store)
				},
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) error {
			command := i.Command
			if len(command.Args) == 0 {
				return runInteractiveFlow(ctx, registry, store)
			}

			parts := make([]string, 0, len(command.Args))
			for _, arg := range command.Args {
				parts = append(parts, strings.TrimSpace(arg.Value.String()))
			}

			state, err := store.Load()
			if err != nil {
				return err
			}

			if err := executeWithAliases(ctx, registry, state, parts); err != nil {
				return fmt.Errorf("%w\nTry: fastgit ggc list", err)
			}

			return nil
		},
	}
}
