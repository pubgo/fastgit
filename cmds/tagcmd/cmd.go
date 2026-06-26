package tagcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	semver "github.com/hashicorp/go-version"
	"github.com/pubgo/dix/v2"
	"github.com/pubgo/dix/v2/dixcontext"
	"github.com/pubgo/fastgit/cmds/fastcommitcmd"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/pathutil"
	"github.com/pubgo/funk/v2/recovery"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/redant"
	"github.com/samber/lo"
	"github.com/yarlson/tap"

	"github.com/pubgo/fastgit/utils"
	"github.com/pubgo/fastgit/utils/fzfutil"
)

type cmdParams struct {
	OpenaiClient *utils.OpenaiClient
	CommitCfg    []*fastcommitcmd.Config
}

func New() *redant.Command {
	var flags = new(struct {
		fastCommit bool
	})

	return &redant.Command{
		Use:   "tag",
		Short: "gen tag and push origin",
		Children: []*redant.Command{
			{
				Use:   "list",
				Short: "list all tags",
				Handler: func(ctx context.Context, command *redant.Invocation) error {
					utils.Spin("fetch git tag: ", func() (r result.Result[any]) {
						utils.GitFetchAll(ctx)
						return
					})

					var tagText = strings.TrimSpace(utils.ShellExecOutput(ctx, "git", "tag", "-n", "--sort=-committerdate").Unwrap())
					tag, err := fzfutil.SelectWithFzf(ctx, strings.NewReader(tagText))
					if err != nil {
						return err
					}

					fmt.Println(tag)
					return nil
				},
			},
		},
		Options: []redant.Option{
			{
				Flag:        "fast",
				Description: "Quickly generate tag.",
				Value:       redant.BoolOf(&flags.fastCommit),
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) error {
			defer recovery.Exit()

			di := dixcontext.Get(ctx)
			var params cmdParams
			params = dix.Inject(di, params)

			utils.LogConfigAndBranch()
			utils.Spin("fetch git tag: ", func() (r result.Result[any]) {
				utils.GitFetchAll(ctx)
				return
			})

			if flags.fastCommit {
				tags := utils.GetAllGitTags(ctx)
				sort.Slice(tags, func(i, j int) bool { return tags[i].GreaterThan(tags[j]) })

				selectTags := lo.Map(tags, func(item *semver.Version, _ int) tap.SelectOption[*semver.Version] {
					return tap.SelectOption[*semver.Version]{
						Value: item,
						Label: item.Original(),
					}
				})
				if len(selectTags) > 10 {
					selectTags = selectTags[:10]
				}

				tagName := "v0.0.1"
				if len(selectTags) > 0 {
					tagResult := tap.Select[*semver.Version](ctx, tap.SelectOptions[*semver.Version]{
						Message: "git tag(enter):",
						Options: selectTags,
					})
					if tagResult == nil {
						return nil
					}
					tagName = tagResult.Original()
				}

				tagName = tap.Text(ctx, tap.TextOptions{
					Message:      "git tag(enter):",
					InitialValue: tagName,
					DefaultValue: tagName,
					Placeholder:  "enter git tag",
					Validate: func(s string) error {
						if !strings.HasPrefix(s, "v") {
							return fmt.Errorf("tag name must start with v")
						}
						if _, err := semver.NewSemver(s); err != nil {
							return fmt.Errorf("tag is invalid, tag=%s err=%w", s, err)
						}
						return nil
					},
				})
				if tagName == "" {
					return fmt.Errorf("tag name is empty")
				}
				return validateAndPublishTag(ctx, tagName, ".version/VERSION", params.CommitCfg)
			}

			p := tea.NewProgram(initialModel())
			m := assert.Must1(p.Run()).(model)
			selected := strings.TrimSpace(m.selected)
			if selected == "" {
				return nil
			}

			tags := utils.GetAllGitTags(ctx)
			verFile := ".version/VERSION"
			var ver *semver.Version
			if selected != envRelease {
				ver = utils.GetNextTag(selected, tags)
			} else {
				if pathutil.IsExist(verFile) {
					ver = lo.Must(semver.NewSemver(strings.TrimSpace(string(lo.Must1(os.ReadFile(verFile))))))
				} else {
					ver = utils.GetNextReleaseTag(tags)
				}
				ver = ver.Core()
			}

			tagName := "v" + strings.TrimPrefix(ver.Original(), "v")
			p1 := tea.NewProgram(InitialTextInputModel(tagName))
			m1 := assert.Must1(p1.Run()).(model2)
			if m1.exit {
				return nil
			}

			tagName = m1.Value()
			return validateAndPublishTag(ctx, tagName, verFile, params.CommitCfg)
		},
	}
}

func validateAndPublishTag(ctx context.Context, tagName, verFile string, commitCfg []*fastcommitcmd.Config) error {
	ver, err := semver.NewVersion(tagName)
	if err != nil {
		return errors.Errorf("tag name is not valid: %s", tagName)
	}

	if utils.IsDirty().Unwrap() {
		return errors.New("working tree has uncommitted changes, please commit or stash before tagging")
	}

	if err := ensureVersionAligned(verFile, ver, commitCfg); err != nil {
		return err
	}

	return publishTag(ctx, tagName)
}

func ensureVersionAligned(verFile string, tag *semver.Version, commitCfg []*fastcommitcmd.Config) error {
	needsVersionAlign := false
	for _, cfg := range commitCfg {
		if cfg.GenVersion {
			needsVersionAlign = true
			break
		}
	}

	if !needsVersionAlign || !pathutil.IsExist(verFile) {
		return nil
	}

	raw := strings.TrimSpace(string(lo.Must1(os.ReadFile(verFile))))
	if raw == "" {
		return nil
	}

	fileVer, err := semver.NewVersion(raw)
	if err != nil {
		return errors.Errorf("%s content is invalid semver: %s", verFile, raw)
	}

	if fileVer.Core().String() != tag.Core().String() {
		return errors.Errorf("%s (%s) is not aligned with tag core (%s), please update and commit first", verFile, fileVer.Core().String(), tag.Core().String())
	}

	return nil
}

func publishTag(ctx context.Context, tagName string) error {
	exists, err := remoteTagExists(ctx, tagName)
	if err != nil {
		return err
	}
	if exists {
		return errors.Errorf("remote tag already exists: %s", tagName)
	}

	if localTagExists(tagName) {
		return errors.Errorf("local tag already exists: %s", tagName)
	}

	if err := utils.ShellExec(ctx, "git", "tag", tagName); err != nil {
		return err
	}
	return utils.ShellExec(ctx, "git", "push", "origin", tagName)
}

func remoteTagExists(ctx context.Context, tagName string) (bool, error) {
	ref := "refs/tags/" + tagName
	r := utils.ShellExecOutput(ctx, "git", "ls-remote", "--tags", "origin", ref)
	if err := r.GetErr(); err != nil {
		return false, err
	}
	return strings.TrimSpace(r.Unwrap()) != "", nil
}

func localTagExists(tagName string) bool {
	cmd := exec.Command("git", "rev-parse", "-q", "--verify", "refs/tags/"+tagName)
	return cmd.Run() == nil
}
