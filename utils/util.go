package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bitfield/script"
	"github.com/briandowns/spinner"
	semver "github.com/hashicorp/go-version"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/recovery"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/funk/v2/typex"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/tidwall/match"
	_ "github.com/tidwall/match"
	"mvdan.cc/sh/v3/shell"

	"github.com/pubgo/fastcommit/configs"
)

func GetAllRemoteTags(ctx context.Context) []*semver.Version {
	log.Info().Msg("get all remote tags")
	output := ShellExecOutput(ctx, "git", "ls-remote", "--tags", "origin").Unwrap()
	return lo.Map(strings.Split(output, "\n"), func(item string, index int) *semver.Version {
		item = strings.TrimSpace(item)
		if !strings.HasPrefix(item, "refs/tags/") {
			return nil
		}

		item = strings.TrimPrefix(item, "refs/tags/")
		if !strings.HasPrefix(item, "v") {
			return nil
		}

		vv, err := semver.NewSemver(item)
		if err != nil {
			log.Err(err).Str("tag", item).Msg("failed to parse git tag")
			assert.Must(err)
		}
		return vv
	})
}

func GetAllGitTags(ctx context.Context) []*semver.Version {
	log.Info().Msg("get all tags")
	var tagText = strings.TrimSpace(ShellExecOutput(ctx, "git", "tag").Unwrap())
	var tags = strings.Split(tagText, "\n")
	var versions = make([]*semver.Version, 0, len(tags))

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if !strings.HasPrefix(tag, "v") {
			continue
		}

		vv, err := semver.NewSemver(tag)
		if err != nil {
			log.Err(err).Str("tag", tag).Msg("failed to parse git tag")
			assert.Must(err)
		}
		versions = append(versions, vv)
	}
	return versions
}

func GetCurMaxVer(ctx context.Context) *semver.Version {
	tags := GetAllGitTags(ctx)
	return typex.DoBlock1(func() *semver.Version {
		return lo.MaxBy(tags, func(a *semver.Version, b *semver.Version) bool { return a.Compare(b) > 0 })
	})
}

func GetNextReleaseTag(tags []*semver.Version) *semver.Version {
	if len(tags) == 0 {
		return semver.Must(semver.NewSemver("v0.0.1"))
	}

	var curMaxVer = typex.DoBlock1(func() *semver.Version {
		return lo.MaxBy(tags, func(a *semver.Version, b *semver.Version) bool { return a.Compare(b) > 0 })
	})

	if curMaxVer.Prerelease() == "" {
		segments := curMaxVer.Core().Segments()
		return assert.Must1(semver.NewSemver(fmt.Sprintf("v%d.%d.%d", segments[0], segments[1], segments[2]+1)))
	}

	return curMaxVer.Core()
}

func GetNextTag(pre string, tags []*semver.Version) *semver.Version {
	if len(tags) == 0 {
		return semver.Must(semver.NewSemver("v0.0.1"))
	}

	var maxVer = GetNextGitMaxTag(tags)
	var curMaxVer = typex.DoBlock1(func() *semver.Version {
		tags = lo.Filter(tags, func(item *semver.Version, index int) bool { return strings.Contains(item.String(), pre) })
		return lo.MaxBy(tags, func(a *semver.Version, b *semver.Version) bool { return a.Compare(b) > 0 })
	})

	var ver string
	if curMaxVer != nil && curMaxVer.Core().GreaterThanOrEqual(maxVer) {
		ver = strings.ReplaceAll(curMaxVer.Prerelease(), fmt.Sprintf("%s.", pre), "")
		ver = fmt.Sprintf("v%s-%s.%d", curMaxVer.Core().String(), pre, assert.Must1(strconv.Atoi(ver))+1)
	} else {
		ver = fmt.Sprintf("v%s-%s.1", maxVer.Core().String(), pre)
	}
	return assert.Must1(semver.NewSemver(ver))
}

func GetNextGitMaxTag(tags []*semver.Version) *semver.Version {
	maxVer := semver.Must(semver.NewVersion("v0.0.1"))
	if len(tags) == 0 {
		return maxVer
	}

	for _, tag := range tags {
		if maxVer.Compare(tag) >= 0 {
			continue
		}

		maxVer = tag
	}

	segments := maxVer.Segments()
	v3Segment := lo.If(strings.Contains(maxVer.String(), "-"), segments[2]).Else(segments[2] + 1)

	return semver.Must(semver.NewVersion(fmt.Sprintf("v%d.%d.%d", segments[0], segments[1], v3Segment)))
}

func UsageDesc(format string, args ...interface{}) string {
	s := fmt.Sprintf(format, args...)
	return strings.ToUpper(s[0:1]) + s[1:]
}

func Context() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
			cancel()
		}
	}()
	return ctx
}

func IsHelp() bool {
	help := strings.TrimSpace(os.Args[len(os.Args)-1])
	if strings.HasSuffix(help, "--help") || strings.HasSuffix(help, "-h") {
		return true
	}
	return false
}

func GitPush(ctx context.Context, args ...string) string {
	now := time.Now()
	args = append([]string{"git", "push"}, args...)
	output := result.Async(func() result.Result[string] { return ShellExecOutput(ctx, args...) })
	time.Sleep(time.Millisecond * 20)

	spin := spinner.New(spinner.CharSets[35], 100*time.Millisecond, func(s *spinner.Spinner) {
		s.Prefix = strings.Join(args, " ") + ":"
	})
	spin.Start()
	res := output.Await(ctx).Unwrap()
	spin.Stop()
	if res != "" {
		log.Info().Str("dur", time.Since(now).String()).Msgf("shell result: \n%s\n", res)
	}
	return res
}

func ShellExec(ctx context.Context, args ...string) (err error) {
	defer result.RecoveryErr(&err)
	now := time.Now()
	res := ShellExecOutput(ctx, args...).Unwrap()

	if res != "" {
		log.Info().Str("dur", time.Since(now).String()).Msgf("shell result: \n%s\n", res)
	}

	return nil
}

func ShellExecOutput(ctx context.Context, args ...string) (r result.Result[string]) {
	defer result.Recovery(&r, func(err error) error {
		if exitErr, ok := errors.AsA[exec.ExitError](err); ok && exitErr.String() == "signal: interrupt" {
			os.Exit(1)
		}

		return err
	})

	sh := getShell()
	if sh != "" {
		args = []string{sh, "-c", fmt.Sprintf(`'%s'`, strings.Join(args, " "))}
	}

	cmdLine := strings.TrimSpace(strings.Join(args, " "))
	log.Info().Msgf("shell: %s", cmdLine)

	args = result.Wrap(shell.Fields(cmdLine, nil)).UnwrapOrLog(func(e *zerolog.Event) {
		e.Str("shell", cmdLine)
	})
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil && !IsOsExit(err) {
		log.Err(err, ctx).Msg("git error\n" + string(output))
		return r.WithErr(err)
	}

	return r.WithValue(strings.TrimSpace(string(output)))
}

func IsRemoteTagExist(err string) bool {
	return strings.Contains(err, "[rejected]") && strings.Contains(err, "tag already exists")
}

func IsRemotePushCommitFailed(err string) bool {
	return strings.Contains(err, "[rejected]") && strings.Contains(err, "failed to push some refs to")
}

func Spin[T any](name string, do func() result.Result[T]) (r result.Result[T]) {
	defer result.Recovery(&r)
	s := spinner.New(spinner.CharSets[35], 100*time.Millisecond, func(s *spinner.Spinner) { s.Prefix = name })
	s.Start()
	defer s.Stop()
	return do()
}

// Your branch and 'origin/fix/version' have diverged,
// and have 1 and 1 different commits each, respectively.
//
//	(use "git pull" if you want to integrate the remote branch with yours)
//
// nothing to commit, working tree clean

func PreGitPush(ctx context.Context) string {
	defer recovery.Exit()

	isDirty := IsDirty().Unwrap()
	if isDirty {
		return ""
	}

	res := ShellExecOutput(ctx, "git", "status").Unwrap()
	needPush := strings.Contains(res, "Your branch is ahead of") && strings.Contains(res, "(use \"git push\" to publish your local commits)")
	if !needPush {
		needPush =
			match.Match(res, "*Your branch and '*' have diverged*") &&
				strings.Contains(ShellExecOutput(ctx, "git", "reflog", "-1").Unwrap(), "(amend)")
	}

	if !needPush {
		return ""
	}

	return GitPush(ctx, "--force-with-lease", "origin", GetBranchName())
}

var GetBranchName = sync.OnceValue(func() string { return GetCurrentBranch().Unwrap() })

func LogConfigAndBranch() {
	log.Info().Msgf("branch: %s", GetBranchName())
	log.Info().Msgf("config: %s", configs.GetConfigPath())
	log.Info().Msgf("local: %s", configs.GetLocalEnvPath())
	log.Info().Msgf("env: %s", configs.GetEnvPath())
	log.Info().Msgf("repo: %s", configs.GetRepoPath())
}

func Run(executors ...func() error) result.Error {
	for _, executor := range executors {
		if err := executor(); err != nil {
			return result.ErrOf(errors.WrapCaller(err, 1))
		}
	}
	return result.Error{}
}

func getShell() string {
	sh := "bash"
	_, err := exec.LookPath(sh)
	if err == nil {
		return sh
	}

	sh = "sh"
	_, err = exec.LookPath(sh)
	if err == nil {
		return sh
	}

	return ""
}

func IsStatusNeedPush(msg string) bool {
	var pattern = `
*Your branch is ahead of '*' by * commits.
  (use "git push" to publish your local commits)*
`

	return match.Match(msg, pattern)
}

var editors = []string{"zed", "subl", "vim", "code", "open"}

func GetEditor() (r result.Result[string]) {
	for _, editor := range editors {
		_, err := exec.LookPath(editor)
		if err == nil {
			return r.WithValue(editor)
		}
	}
	return r.WithErr(errors.Errorf("no editor found in %q", editors))
}

func Edit(editPath string) {
	log.Info().Msgf("edit path: %s", editPath)
	editor := GetEditor().Unwrap()
	path := assert.Exit1(filepath.Abs(editPath))
	shellData := fmt.Sprintf(`%s "%s"`, editor, path)
	log.Info().Msg(shellData)
	assert.Exit1(script.Exec(shellData).Stdout())
}

func IsOsExit(err error) bool { return IsErrExit1(err) || IsErrSignalInterrupt(err) }

func IsErrExit1(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "exit status 1")
}

func IsErrSignalInterrupt(err error) bool {
	if err == nil {
		return false
	}

	exitErr, ok := errors.AsA[exec.ExitError](err)
	return ok && strings.Contains(exitErr.String(), "signal: interrupt")
}
