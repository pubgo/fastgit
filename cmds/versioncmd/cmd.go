package versioncmd

import (
	"context"
	"fmt"

	"github.com/pubgo/funk/v2/buildinfo/version"
	"github.com/pubgo/funk/v2/recovery"
	"github.com/pubgo/funk/v2/running"
	"github.com/pubgo/redant"
)

func New() *redant.Command {
	return &redant.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "version info",
		Handler: func(ctx context.Context, i *redant.Invocation) error {
			defer recovery.Exit()
			fmt.Println("project:", version.Project())
			fmt.Println("version:", version.Version())
			fmt.Println("release:", version.ReleaseVersion())
			fmt.Println("commit-id:", version.CommitID())
			fmt.Println("build-time:", version.BuildTime())
			fmt.Println("device-id:", running.DeviceID)
			return nil
		},
	}
}
