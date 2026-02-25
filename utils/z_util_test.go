package utils_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pubgo/fastcommit/utils"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/match"
)

func TestErrTagExists(t *testing.T) {
	var errMsg = `
To github.com:pubgo/funk.git
 ! [rejected]          v0.5.69-alpha.23 -> v0.5.69-alpha.23 (already exists)
error: failed to push some refs to 'github.com:pubgo/funk.git'
hint: Updates were rejected because the tag already exists in the remote.`
	assert.Equal(t, utils.IsRemoteTagExist(errMsg), true)
}

func TestMatch(t *testing.T) {
	var txt = `123  Your branch and 'origin/feat/genai' have diverged 123`
	t.Log(match.Match(txt, fmt.Sprintf("*Your branch and '*feat/genai' have diverged*")))

	t.Log(strings.Contains(utils.ShellExecOutput(context.Background(), "git", "reflog", "-1").Unwrap(), "(amend)"))
}
