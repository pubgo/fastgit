package prcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/yarlson/tap"
)

func confirmMerge(ctx context.Context, gh *GhClient, method string) error {
	method = strings.TrimSpace(method)
	if method == "" {
		method = "squash"
	}
	if view, err := gh.ViewPR(ctx); err == nil && strings.TrimSpace(view) != "" {
		fmt.Println(view)
	}
	answer := strings.ToLower(strings.TrimSpace(tap.Text(ctx, tap.TextOptions{
		Message:      fmt.Sprintf("Merge PR with %s? (y/N):", method),
		DefaultValue: "n",
		InitialValue: "n",
		Placeholder:  "y or n",
	})))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("merge cancelled")
	}
	return nil
}
