package repoconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCommitMessage(t *testing.T) {
	bundle := Bundle{Commit: CommitSettings{MaxLength: 72}}
	bundle.Policy.Commit.Conventional = true

	require.NoError(t, bundle.ValidateCommitMessage("feat: add conflict command"))
	require.Error(t, bundle.ValidateCommitMessage("bad message"))

	bundle.Commit.RequireScope = true
	require.Error(t, bundle.ValidateCommitMessage("feat: missing scope"))
	require.NoError(t, bundle.ValidateCommitMessage("feat(core): add scope"))
}

func TestMatchesSensitivePath(t *testing.T) {
	bundle := Bundle{Policy: Policy{SensitivePaths: []string{".env", "**/*secret*"}}}
	require.True(t, bundle.MatchesSensitivePath(".env"))
	require.True(t, bundle.MatchesSensitivePath("config/secret.yaml"))
}

func TestCheckCommitMessageEnforce(t *testing.T) {
	bundle := Bundle{
		Commit: CommitSettings{MaxLength: 72},
		Policy: Policy{Enforce: true},
	}
	bundle.Policy.Commit.Conventional = true

	require.Error(t, bundle.CheckCommitMessage("bad message", false))
	require.NoError(t, bundle.CheckCommitMessage("bad message", true))
	require.NoError(t, bundle.CheckCommitMessage("feat: ok", false))
}

func TestCheckBranchEnforce(t *testing.T) {
	bundle := Bundle{Policy: Policy{Enforce: true}}
	bundle.Policy.Branch.Pattern = `^feature/`

	require.Error(t, bundle.CheckBranch("main", false))
	require.NoError(t, bundle.CheckBranch("main", true))
}

func TestValidateBranch(t *testing.T) {
	bundle := Bundle{Policy: Policy{}}
	bundle.Policy.Branch.Pattern = `^feature/[a-z0-9-]+$`
	require.NoError(t, bundle.ValidateBranch("feature/add-conflict"))
	require.Error(t, bundle.ValidateBranch("main"))
}
