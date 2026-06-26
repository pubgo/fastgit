package main

import (
	"fmt"
	"os"

	"github.com/pubgo/fastgit/cmds/copilotcmd"
)

func main() {
	if err := copilotcmd.New().Invoke().WithOS().Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
