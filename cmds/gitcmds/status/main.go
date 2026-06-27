package main

import (
	"os"

	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/pretty"

	"github.com/go-git/go-git/v6"
	examples "github.com/go-git/go-git/v6/_examples"
)

// Basic example of how to commit changes to the current branch to an existing
// repository.
func main() {
	directory := assert.Exit1(os.Getwd())

	// Opens an already existing repository.
	r, err := git.PlainOpen(directory)
	examples.CheckIfError(err)

	w, err := r.Worktree()
	examples.CheckIfError(err)

	// We can verify the current status of the worktree using the method Status.
	examples.Info("git status --porcelain")
	status, err := w.StatusWithOptions(git.StatusOptions{Strategy: git.Preload})
	examples.CheckIfError(err)
	pretty.Println(status)
	pretty.Println(status.String())
}
