package ggccmd

import (
	"context"
	"fmt"
	"strings"
)

const maxAliasDepth = 8

func executeWithAliases(ctx context.Context, registry *Registry, state *GGCState, args []string) error {
	return executeWithAliasesDepth(ctx, registry, state, args, 0)
}

func executeWithAliasesDepth(ctx context.Context, registry *Registry, state *GGCState, args []string, depth int) error {
	if len(args) == 0 {
		return nil
	}
	if depth > maxAliasDepth {
		return fmt.Errorf("alias expansion too deep")
	}

	name := args[0]
	if alias, ok := state.Aliases[name]; ok {
		return executeAlias(ctx, registry, state, alias, args[1:], depth+1)
	}

	return registry.Execute(ctx, args)
}

func executeAlias(ctx context.Context, registry *Registry, state *GGCState, alias AliasDef, aliasArgs []string, depth int) error {
	templates := alias.Templates()
	if len(templates) == 0 {
		return fmt.Errorf("empty alias")
	}

	for _, tpl := range templates {
		expanded, err := expandAliasTemplate(tpl, aliasArgs)
		if err != nil {
			return err
		}
		tokens, err := splitCommandLine(expanded)
		if err != nil {
			return err
		}
		if len(tokens) == 0 {
			continue
		}
		if err := executeWithAliasesDepth(ctx, registry, state, tokens, depth); err != nil {
			return fmt.Errorf("alias step %q failed: %w", strings.Join(tokens, " "), err)
		}
	}
	return nil
}

func buildInteractiveEntries(registry *Registry, state *GGCState) []CommandEntry {
	entries := registry.List()
	for _, name := range sortedAliasNames(state.Aliases) {
		alias := state.Aliases[name]
		entries = append(entries, CommandEntry{
			Key:         name,
			Usage:       name + alias.UsageSuffix(),
			Description: "Alias: " + alias.Summary(),
		})
	}
	return entries
}
