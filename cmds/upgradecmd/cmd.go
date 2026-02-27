package upgradecmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-version"
	"github.com/olekukonko/tablewriter"
	"github.com/pubgo/funk/v2/assert"
	"github.com/pubgo/funk/v2/errors"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/pretty"
	"github.com/pubgo/funk/v2/result"
	"github.com/pubgo/redant"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/yarlson/tap"

	"github.com/pubgo/fastgit/utils/githubclient"
)

func New() *redant.Command {
	return &redant.Command{
		Use:   "upgrade",
		Short: "self upgrade management",
		Children: []*redant.Command{
			{
				Use: "list",
				Handler: func(ctx context.Context, i *redant.Invocation) error {
					client := githubclient.NewPublicRelease("pubgo", "fastgit")
					releases := assert.Must1(client.List(ctx))

					tt := tablewriter.NewWriter(os.Stdout)
					tt.Header([]string{"Name", "Size", "Url"})

					for _, r := range releases {
						for _, a := range githubclient.GetAssets(r) {
							if a.IsChecksumFile() {
								continue
							}

							if a.OS != runtime.GOOS {
								continue
							}

							if a.Arch != runtime.GOARCH {
								continue
							}

							assert.Must(tt.Append([]string{
								a.Name,
								githubclient.GetSizeFormat(a.Size),
								a.URL,
							}))
						}
					}
					return tt.Render()
				},
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) (gErr error) {
			defer result.RecoveryErr(&gErr, func(err error) error {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				pretty.Println(err)
				return err
			})

			client := githubclient.NewPublicRelease("pubgo", "fastgit")
			r := assert.Must1(client.List(ctx))

			assets := githubclient.GetAssetList(r)
			assets = lo.Filter(assets, func(item githubclient.Asset, index int) bool {
				return !item.IsChecksumFile() && item.OS == runtime.GOOS && item.Arch == runtime.GOARCH
			})
			sort.Slice(assets, func(i, j int) bool {
				return assert.Must1(version.NewSemver(assets[i].Name)).GreaterThan(lo.Must(version.NewSemver(assets[j].Name)))
			})

			if len(assets) > 20 {
				assets = assets[:20]
			}

			versionName := tap.Select[string](ctx, tap.SelectOptions[string]{
				Message: "Which version do you prefer?",
				Options: lo.Map(assets, func(item githubclient.Asset, index int) tap.SelectOption[string] {
					return tap.SelectOption[string]{
						Value: item.Name,
						Label: item.Name,
					}
				}),
			})

			if versionName == "" {
				return nil
			}

			log.Info(ctx).Msgf("You chose: %s", versionName)

			asset, ok := lo.Find(assets, func(item githubclient.Asset) bool { return item.Name == versionName })
			assert.If(!ok, "%s not found", versionName)
			var downloadURL = asset.URL

			downloadDir := filepath.Join(os.TempDir(), "fastgit")
			pwd := assert.Must1(os.Getwd())

			execFile := filepath.Base(os.Args[0])
			execFile = assert.Must1(exec.LookPath(execFile))

			log.Info().Func(func(e *zerolog.Event) {
				e.Str("download_dir", downloadDir)
				e.Str("pwd", pwd)
				e.Str("exec_file", execFile)
				e.Msgf("start download %s", downloadURL)
			})

			c := &getter.Client{
				Ctx:              ctx,
				Src:              downloadURL,
				Dst:              downloadDir,
				Pwd:              pwd,
				Mode:             getter.ClientModeDir,
				ProgressListener: defaultProgressBar,
			}
			assert.Must(c.Get())
			assert.Must(os.Rename(downloadDir+"/fastgit", execFile))

			return nil
		},
	}
}
