package ggccmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/pubgo/redant"
)

func New() *redant.Command {
	registry := NewRegistry()

	return &redant.Command{
		Use:   "ggc",
		Short: "Unified Git command surface inspired by ggc",
		Children: []*redant.Command{
			{
				Use:   "list",
				Short: "List supported unified commands",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					for _, entry := range registry.List() {
						fmt.Printf("- %-28s %s\n", entry.Usage, entry.Description)
					}
					return nil
				},
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) error {
			command := i.Command
			if len(command.Args) == 0 {
				fmt.Println("Usage: fastgit ggc <command...>")
				fmt.Println("Tip: fastgit ggc list")
				return nil
			}

			parts := make([]string, 0, len(command.Args))
			for _, arg := range command.Args {
				parts = append(parts, strings.TrimSpace(arg.Value.String()))
			}

			if err := registry.Execute(ctx, parts); err != nil {
				return fmt.Errorf("%w\nTry: fastgit ggc list", err)
			}

			return nil
		},
	}
}
