package chglogcmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	semver "github.com/hashicorp/go-version"
)

const defaultInitialVersion = "v0.1.0"

var standardSections = []string{"新增", "修复", "变更", "文档"}

type changelogPaths struct {
	RepoRoot            string
	VersionDir          string
	VersionFile         string
	ChangelogDir        string
	UnreleasedFile      string
	ReadmeFile          string
	GitHubDir           string
	PromptsDir          string
	InstructionsDir     string
	ChangelogPromptFile string
	ChangelogRulesFile  string
	ReleaseRulesFile    string
}

type scaffoldOptions struct {
	Version                string
	Force                  bool
	CreateVersionIfMissing bool
}

type scaffoldResult struct {
	Created []string
	Updated []string
}

type releaseOptions struct {
	Version     string
	NextVersion string
	Bump        string
	DryRun      bool
}

type releaseResult struct {
	CreatedFiles []string
	UpdatedFiles []string
	NextVersion  string
}

func resolveRepoRoot(input string) (string, error) {
	path := strings.TrimSpace(input)
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve repo path: %w", err)
	}
	return path, nil
}

func resolveExistingGitRepo(input string) (string, error) {
	repoRoot, err := resolveRepoRoot(input)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve git repository(%s): %w", repoRoot, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func buildPaths(repoRoot string) changelogPaths {
	versionDir := filepath.Join(repoRoot, ".version")
	changelogDir := filepath.Join(versionDir, "changelog")
	return changelogPaths{
		RepoRoot:            repoRoot,
		VersionDir:          versionDir,
		VersionFile:         filepath.Join(versionDir, "VERSION"),
		ChangelogDir:        changelogDir,
		UnreleasedFile:      filepath.Join(changelogDir, "Unreleased.md"),
		ReadmeFile:          filepath.Join(changelogDir, "README.md"),
		GitHubDir:           filepath.Join(repoRoot, ".github"),
		PromptsDir:          filepath.Join(repoRoot, ".github", "prompts"),
		InstructionsDir:     filepath.Join(repoRoot, ".github", "instructions"),
		ChangelogPromptFile: filepath.Join(repoRoot, ".github", "prompts", "changelog.prompt.md"),
		ChangelogRulesFile:  filepath.Join(repoRoot, ".github", "instructions", "changelog.instructions.md"),
		ReleaseRulesFile:    filepath.Join(repoRoot, ".github", "instructions", "release.instructions.md"),
	}
}

func ensureChangelogScaffold(repoRoot string, opts scaffoldOptions) (scaffoldResult, error) {
	paths := buildPaths(repoRoot)
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = defaultInitialVersion
	}
	if err := validateVersion(version); err != nil {
		return scaffoldResult{}, err
	}

	if err := os.MkdirAll(paths.ChangelogDir, 0o755); err != nil {
		return scaffoldResult{}, fmt.Errorf("create changelog directory: %w", err)
	}
	if err := os.MkdirAll(paths.PromptsDir, 0o755); err != nil {
		return scaffoldResult{}, fmt.Errorf("create prompts directory: %w", err)
	}
	if err := os.MkdirAll(paths.InstructionsDir, 0o755); err != nil {
		return scaffoldResult{}, fmt.Errorf("create instructions directory: %w", err)
	}

	result := scaffoldResult{}

	if !fileExists(paths.VersionFile) && opts.CreateVersionIfMissing {
		if err := os.MkdirAll(paths.VersionDir, 0o755); err != nil {
			return scaffoldResult{}, fmt.Errorf("create version directory: %w", err)
		}
		if err := os.WriteFile(paths.VersionFile, []byte(version+"\n"), 0o644); err != nil {
			return scaffoldResult{}, fmt.Errorf("write VERSION: %w", err)
		}
		result.Created = append(result.Created, paths.VersionFile)
	} else if fileExists(paths.VersionFile) && opts.Force {
		if err := os.WriteFile(paths.VersionFile, []byte(version+"\n"), 0o644); err != nil {
			return scaffoldResult{}, fmt.Errorf("rewrite VERSION: %w", err)
		}
		result.Updated = append(result.Updated, paths.VersionFile)
	}

	state, err := writeManagedFile(paths.UnreleasedFile, renderUnreleasedTemplate(), opts.Force)
	if err != nil {
		return scaffoldResult{}, err
	}
	recordScaffoldState(&result, paths.UnreleasedFile, state)

	readmeContent, err := renderChangelogReadme(paths.ChangelogDir)
	if err != nil {
		return scaffoldResult{}, err
	}
	state, err = writeManagedFile(paths.ReadmeFile, readmeContent, opts.Force)
	if err != nil {
		return scaffoldResult{}, err
	}
	recordScaffoldState(&result, paths.ReadmeFile, state)

	state, err = writeManagedFile(paths.ChangelogPromptFile, renderRepoChangelogPromptTemplate(), opts.Force)
	if err != nil {
		return scaffoldResult{}, err
	}
	recordScaffoldState(&result, paths.ChangelogPromptFile, state)

	state, err = writeManagedFile(paths.ChangelogRulesFile, renderRepoChangelogRulesTemplate(), opts.Force)
	if err != nil {
		return scaffoldResult{}, err
	}
	recordScaffoldState(&result, paths.ChangelogRulesFile, state)

	state, err = writeManagedFile(paths.ReleaseRulesFile, renderRepoReleaseRulesTemplate(), opts.Force)
	if err != nil {
		return scaffoldResult{}, err
	}
	recordScaffoldState(&result, paths.ReleaseRulesFile, state)

	return result, nil
}

func recordScaffoldState(result *scaffoldResult, path, state string) {
	if result == nil {
		return
	}
	switch state {
	case "created":
		result.Created = append(result.Created, path)
	case "updated":
		result.Updated = append(result.Updated, path)
	}
}

func writeManagedFile(path, content string, force bool) (string, error) {
	if fileExists(path) && !force {
		return "skipped", nil
	}
	state := "created"
	if fileExists(path) {
		state = "updated"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return state, nil
}

func releaseChangelog(repoRoot string, opts releaseOptions) (releaseResult, error) {
	paths := buildPaths(repoRoot)
	if _, err := ensureChangelogScaffold(repoRoot, scaffoldOptions{Version: defaultInitialVersion, CreateVersionIfMissing: true}); err != nil {
		return releaseResult{}, err
	}

	currentVersion := strings.TrimSpace(opts.Version)
	if currentVersion == "" {
		content, err := os.ReadFile(paths.VersionFile)
		if err != nil {
			return releaseResult{}, fmt.Errorf("read VERSION: %w", err)
		}
		currentVersion = strings.TrimSpace(string(content))
	}
	if err := validateVersion(currentVersion); err != nil {
		return releaseResult{}, err
	}

	targetFile := filepath.Join(paths.ChangelogDir, currentVersion+".md")
	if fileExists(targetFile) {
		return releaseResult{}, fmt.Errorf("release file already exists: %s", targetFile)
	}

	unreleasedContent, err := os.ReadFile(paths.UnreleasedFile)
	if err != nil {
		return releaseResult{}, fmt.Errorf("read Unreleased.md: %w", err)
	}

	sections := parseStandardSections(string(unreleasedContent))
	if !hasMeaningfulEntries(sections) {
		return releaseResult{}, errors.New("Unreleased.md 中没有可发布的变更条目")
	}
	releaseContent := renderReleaseContent(currentVersion, time.Now(), sections)
	nextVersion, err := resolveNextVersion(currentVersion, strings.TrimSpace(opts.NextVersion), strings.TrimSpace(opts.Bump))
	if err != nil {
		return releaseResult{}, err
	}

	created := []string{targetFile}
	updated := []string{paths.UnreleasedFile, paths.ReadmeFile}
	if nextVersion != "" {
		updated = append(updated, paths.VersionFile)
	}

	if opts.DryRun {
		return releaseResult{CreatedFiles: created, UpdatedFiles: updated, NextVersion: nextVersion}, nil
	}

	if err := os.WriteFile(targetFile, []byte(releaseContent), 0o644); err != nil {
		return releaseResult{}, fmt.Errorf("write release file: %w", err)
	}
	if err := os.WriteFile(paths.UnreleasedFile, []byte(renderUnreleasedTemplate()), 0o644); err != nil {
		return releaseResult{}, fmt.Errorf("reset Unreleased.md: %w", err)
	}

	readmeContent, err := renderChangelogReadme(paths.ChangelogDir)
	if err != nil {
		return releaseResult{}, err
	}
	if err := os.WriteFile(paths.ReadmeFile, []byte(readmeContent), 0o644); err != nil {
		return releaseResult{}, fmt.Errorf("update README.md: %w", err)
	}

	if nextVersion != "" {
		if err := os.WriteFile(paths.VersionFile, []byte(nextVersion+"\n"), 0o644); err != nil {
			return releaseResult{}, fmt.Errorf("update VERSION: %w", err)
		}
	}

	return releaseResult{CreatedFiles: created, UpdatedFiles: updated, NextVersion: nextVersion}, nil
}

func renderUnreleasedTemplate() string {
	return strings.TrimSpace(`
# [Unreleased]

> 推荐维护方式：`+"`fastgit changelog draft|release`"+`

## 新增

暂无

## 修复

暂无

## 变更

暂无

## 文档

暂无
`) + "\n"
}

func renderReleaseContent(version string, now time.Time, sections map[string]string) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("# [%s] - %s\n\n", version, now.Format("2006-01-02")))
	for _, title := range standardSections {
		body := normalizeSectionBody(sections[title])
		buf.WriteString("## " + title + "\n\n")
		buf.WriteString(body)
		buf.WriteString("\n\n")
	}
	return buf.String()
}

func parseStandardSections(content string) map[string]string {
	sections := make(map[string]string, len(standardSections))
	for _, title := range standardSections {
		sections[title] = "暂无"
	}

	current := ""
	var buf bytes.Buffer
	flush := func() {
		if current == "" {
			buf.Reset()
			return
		}
		sections[current] = normalizeSectionBody(buf.String())
		buf.Reset()
	}

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "## ") {
			flush()
			title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if containsString(standardSections, title) {
				current = title
			} else {
				current = ""
			}
			continue
		}
		if current == "" {
			continue
		}
		buf.WriteString(rawLine)
		buf.WriteString("\n")
	}
	flush()
	return sections
}

func normalizeSectionBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "暂无"
	}
	if body == "暂无" {
		return body
	}
	return body
}

func hasMeaningfulEntries(sections map[string]string) bool {
	for _, title := range standardSections {
		body := normalizeSectionBody(sections[title])
		if strings.TrimSpace(body) != "" && strings.TrimSpace(body) != "暂无" {
			return true
		}
	}
	return false
}

func renderChangelogReadme(changelogDir string) (string, error) {
	versions, err := listReleaseVersions(changelogDir)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString("# Changelog 索引\n\n")
	buf.WriteString("本目录保存项目变更记录，采用“一个版本一个文件”的方式维护。\n\n")
	buf.WriteString("## 文件约定\n\n")
	buf.WriteString("- `Unreleased.md`：当前开发中变更（待发布）。\n")
	buf.WriteString("- `vX.Y.Z.md`：已发布版本变更（例如 `v0.0.5.md`）。\n\n")
	buf.WriteString("## 当前版本文件\n\n")
	buf.WriteString("- [`Unreleased.md`](Unreleased.md)\n")
	for _, version := range versions {
		buf.WriteString(fmt.Sprintf("- [`%s.md`](%s.md)\n", version, version))
	}
	buf.WriteString("\n## 维护约定\n\n")
	buf.WriteString("- 分类保持：`新增` / `修复` / `变更` / `文档`。\n")
	buf.WriteString("- 发布时将 `Unreleased.md` 内容迁移到新版本文件，并重建空模板。\n")
	buf.WriteString("- 历史版本文件只做勘误，不改写语义与顺序。\n")
	return buf.String(), nil
}

func listReleaseVersions(changelogDir string) ([]string, error) {
	entries, err := os.ReadDir(changelogDir)
	if err != nil {
		return nil, fmt.Errorf("read changelog directory: %w", err)
	}

	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "v") || !strings.HasSuffix(name, ".md") || name == "Unreleased.md" {
			continue
		}
		versions = append(versions, strings.TrimSuffix(name, ".md"))
	}

	sort.SliceStable(versions, func(i, j int) bool {
		vi, errI := semver.NewVersion(versions[i])
		vj, errJ := semver.NewVersion(versions[j])
		if errI != nil || errJ != nil {
			return versions[i] > versions[j]
		}
		return vi.GreaterThan(vj)
	})

	return versions, nil
}

func resolveNextVersion(current, explicit, bump string) (string, error) {
	if strings.TrimSpace(explicit) != "" && strings.TrimSpace(bump) != "" {
		return "", errors.New("--next-version 与 --bump 只能二选一")
	}
	if strings.TrimSpace(explicit) != "" {
		if err := validateVersion(explicit); err != nil {
			return "", err
		}
		return strings.TrimSpace(explicit), nil
	}
	if strings.TrimSpace(bump) == "" {
		return "", nil
	}

	v, err := semver.NewVersion(current)
	if err != nil {
		return "", fmt.Errorf("parse current version %q: %w", current, err)
	}
	segments := v.Segments()
	if len(segments) < 3 {
		return "", fmt.Errorf("invalid version: %s", current)
	}

	switch strings.ToLower(strings.TrimSpace(bump)) {
	case "patch":
		segments[2]++
	case "minor":
		segments[1]++
		segments[2] = 0
	case "major":
		segments[0]++
		segments[1] = 0
		segments[2] = 0
	default:
		return "", fmt.Errorf("unsupported bump level: %s", bump)
	}

	return fmt.Sprintf("v%d.%d.%d", segments[0], segments[1], segments[2]), nil
}

func validateVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("version cannot be empty")
	}
	if _, err := semver.NewVersion(version); err != nil {
		return fmt.Errorf("invalid version %q: %w", version, err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
