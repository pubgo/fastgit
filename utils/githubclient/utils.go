package githubclient

import (
	"regexp"
	"strings"
)

var (
	archRe     = regexp.MustCompile(`(arm64|386|686|amd64|x86_64|aarch64|\barm\b|\b32\b|\b64\b)`)
	posixOSRe  = regexp.MustCompile(`(darwin|linux|(net|free|open)bsd|mac|osx|windows|win)`)
	checksumRe = regexp.MustCompile(`(checksums|sha256sums)`)
)

func getOS(s string) string {
	s = strings.ToLower(s)
	o := posixOSRe.FindString(s)
	if o == "mac" || o == "osx" {
		o = "darwin"
	}

	if o == "win" {
		o = "windows"
	}

	return o
}

func getArch(s string) string {
	s = strings.ToLower(s)
	a := archRe.FindString(s)

	// arch modifications
	switch a {
	case "", "64", "x86_64":
		a = "amd64" // default
	case "32", "686":
		a = "386"
	case "aarch64":
		a = "arm64"
	}
	return a
}
