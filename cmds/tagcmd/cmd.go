package tagcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
			if flags.fastCommit {
				tags := utils.GetAllGitTags(ctx)

				sort.Slice(tags, func(i, j int) bool { return tags[i].GreaterThanOrEqual(tags[j]) })
				selectTags := lo.Map(tags, func(item *semver.Version, index int) tap.SelectOption[*semver.Version] {
					return tap.SelectOption[*semver.Version]{
						Value: item,
						Label: item.Original(),
					}
				})
				selectTags = lo.Chunk(selectTags, 10)[0]

				tagResult := tap.Select[*semver.Version](ctx, tap.SelectOptions[*semver.Version]{
					Message: "git tag(enter):",
					Options: selectTags,
				})

				if tagResult == nil {
					return nil
				}

				tagName := tap.Text(ctx, tap.TextOptions{
					Message:      "git tag(enter):",
					InitialValue: tagResult.Original(),
					DefaultValue: tagResult.Original(),
					Placeholder:  "enter git tag",
					Validate: func(s string) error {
						if !strings.HasPrefix(s, "v") {
							return fmt.Errorf("tag name must start with v")
						}

						_, err := semver.NewSemver(s)
						if err == nil {
							return nil
						}
						return fmt.Errorf("tag is invalid, tag=%s err=%w", s, err)
					},
				})

				if tagName == "" {
					return fmt.Errorf("tag name is empty")
				}

				fmt.Println(utils.GitPushTag(ctx, tagName))
				return nil
			}

			var p = tea.NewProgram(initialModel())
			m := assert.Must1(p.Run()).(model)
			selected := strings.TrimSpace(m.selected)
			if selected == "" {
				return nil
			}

			tags := utils.GetAllGitTags(ctx)

			var ver *semver.Version
			verFile := ".version/VERSION"
			if selected != envRelease {
				//if pathutil.IsExist(verFile) {
				//vv := strings.TrimPrefix(string(lo.Must1(os.ReadFile(verFile))), "v")
				//maxTag := lo.MaxBy(tags, func(a *semver.Version, b *semver.Version) bool { return a.Compare(b) > 0 })
				//if maxTag != nil && maxTag.Core().String() != vv {
				//	log.Warn().Str("max-version", maxTag.Core().String()).Msg("current version is not equal to .version")
				//}

				//tags = lo.Filter(tags, func(item *semver.Version, index int) bool { return item.Core().String() == vv })
				//if len(tags) == 0 {
				//	ver = lo.Must1(semver.NewSemver(fmt.Sprintf("%s-%s.1", lo.Must1(os.ReadFile(verFile)), selected)))
				//} else {
				//	ver = utils.GetNextTag(selected, tags)
				//}
				//} else {
				ver = utils.GetNextTag(selected, tags)
				//}
			} else {
				if pathutil.IsExist(verFile) {
					ver = lo.Must(semver.NewSemver(strings.TrimSpace(string(lo.Must1(os.ReadFile(verFile))))))
				} else {
					ver = utils.GetNextReleaseTag(tags)
				}
				ver = ver.Core()
			}

			tagName := "v" + strings.TrimPrefix(ver.Original(), "v")
			var p1 = tea.NewProgram(InitialTextInputModel(tagName))
			m1 := assert.Must1(p1.Run()).(model2)
			if m1.exit {
				return nil
			}

			tagName = m1.Value()
			ver, err := semver.NewVersion(tagName)
			if err != nil {
				return errors.Errorf("tag name is not valid: %s", tagName)
			}

			for _, cfg := range params.CommitCfg {
				if !cfg.GenVersion {
					continue
				}

				dir := filepath.Dir(verFile)
				if !pathutil.IsDir(dir) {
					_ = pathutil.DeleteFile(dir)
					_ = pathutil.MkDir(dir)
				}

				assert.Exit(os.WriteFile(verFile, []byte("v"+ver.Core().String()+"\n"), 0644))
				break
			}

			isDirty := utils.IsDirty().Unwrap()
			if isDirty {
				preMsg := strings.TrimSpace(utils.ShellExecOutput(ctx, "git", "log", "-1", "--pretty=%B").Unwrap())
				assert.Must(utils.ShellExec(ctx, "git", "add", "-A"))
				utils.ShellExecOutput(ctx, "git", "status").Unwrap()
				assert.Must(utils.ShellExec(ctx, "git", "commit", "--amend", "--no-edit", "-m", strconv.Quote(preMsg)))
				fmt.Println(utils.GitPush(ctx, "--force-with-lease", "origin", utils.GetBranchName()))
			}

			output := utils.GitPushTag(ctx, tagName)
			if utils.IsRemoteTagExist(output) {
				utils.Spin("fetch git tag: ", func() (r result.Result[any]) {
					utils.GitFetchAll(ctx)
					return
				})
			}

			return nil
		},
	}
}
