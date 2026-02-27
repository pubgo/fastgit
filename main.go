package main

import (
	_ "embed"

	"github.com/pubgo/fastgit/bootstrap"
	"github.com/pubgo/funk/v2/buildinfo/version"
	_ "github.com/pubgo/redant"
)

//go:embed .version/VERSION
var release string
var _ = version.SetVersion(release)

func main() {
	bootstrap.Main()
}
