package githubclient

import (
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/google/go-github/v71/github"
)

func GetAssetList(repo []*github.RepositoryRelease) Assets {
	var assetList Assets
	for _, a := range repo {
		assetList = append(assetList, GetAssets(a)...)
	}
	return assetList
}

func GetAssets(repo *github.RepositoryRelease) Assets {
	var assetList Assets
	for _, a := range repo.Assets {
		assetList = append(assetList, Asset{
			Name:      repo.GetTagName(),
			URL:       a.GetBrowserDownloadURL(),
			Type:      a.GetContentType(),
			Size:      a.GetSize(),
			CreatedAt: a.GetCreatedAt().Time,
			OS:        getOS(a.GetName()),
			Arch:      getArch(a.GetName()),

			// maximum file size 64KB
			ChecksumFile: checksumRe.MatchString(strings.ToLower(a.GetName())) && a.GetSize() < 64*1024,
		})
	}
	return assetList
}

type Asset struct {
	Name, OS, Arch, URL, Type string
	Size                      int
	CreatedAt                 time.Time
	ChecksumFile              bool
}

func (a Asset) IsChecksumFile() bool {
	return a.ChecksumFile
}

func (a Asset) Key() string {
	return a.OS + "/" + a.Arch
}

func (a Asset) Is32Bit() bool {
	return a.Arch == "386"
}

func (a Asset) IsMac() bool {
	return a.OS == "darwin"
}
func (a Asset) IsWindows() bool {
	return a.OS == "windows"
}

func (a Asset) IsLinux() bool {
	return a.OS == "linux"
}

func (a Asset) IsMacM1() bool {
	return a.IsMac() && a.Arch == "arm64"
}

type Assets []Asset

func (as Assets) HasM1() bool {
	//detect if we have a native m1 asset
	for _, a := range as {
		if a.IsMacM1() {
			return true
		}
	}
	return false
}

func GetSizeFormat(size int) string {
	return units.HumanSize(float64(size))
}
