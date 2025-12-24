package main

import (
	"context"
	"strings"

	"github.com/pubgo/fastcommit/utils/fzfutil"
	"github.com/pubgo/funk/v2"
	"github.com/pubgo/funk/v2/pretty"
)

func main() {
	pretty.Println(fzfutil.SelectWithFzf(context.Background(), strings.NewReader(strings.Join(funk.ListOf(
		"abc",
		"123",
		"333",
	), "\n"))))
}
