package acpclient

import (
	"strings"

	"github.com/pubgo/funk/v2/buildinfo/version"
)

func versionString() string {
	v := strings.TrimSpace(version.Version())
	if v == "" {
		return "dev"
	}
	return v
}
