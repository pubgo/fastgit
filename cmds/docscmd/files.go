package docscmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type docPaths struct {
	RepoRoot                string
	GitHubDir               string
	PromptsDir              string
	InstructionsDir         string
	DocumentationPromptFile string
	CommitMessagePromptFile string
	DocumentationRulesFile  string
}

type scaffoldOptions struct {
	Force bool
}

type scaffoldResult struct {
	Created []string
	Updated []string
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

func buildPaths(repoRoot string) docPaths {
	return docPaths{
		RepoRoot:                repoRoot,
		GitHubDir:               filepath.Join(repoRoot, ".github"),
		PromptsDir:              filepath.Join(repoRoot, ".github", "prompts"),
		InstructionsDir:         filepath.Join(repoRoot, ".github", "instructions"),
		DocumentationPromptFile: filepath.Join(repoRoot, ".github", "prompts", "documentation.prompt.md"),
		CommitMessagePromptFile: filepath.Join(repoRoot, ".github", "prompts", "commit-message.prompt.md"),
		DocumentationRulesFile:  filepath.Join(repoRoot, ".github", "instructions", "documentation.instructions.md"),
	}
}

func ensureDocumentationScaffold(repoRoot string, opts scaffoldOptions) (scaffoldResult, error) {
	paths := buildPaths(repoRoot)
	if err := os.MkdirAll(paths.PromptsDir, 0o755); err != nil {
		return scaffoldResult{}, fmt.Errorf("create prompts directory: %w", err)
	}
	if err := os.MkdirAll(paths.InstructionsDir, 0o755); err != nil {
		return scaffoldResult{}, fmt.Errorf("create instructions directory: %w", err)
	}

	result := scaffoldResult{}

	state, err := writeManagedFile(paths.DocumentationPromptFile, renderDocumentationPromptTemplate(), opts.Force)
	if err != nil {
		return scaffoldResult{}, err
	}
	recordScaffoldState(&result, paths.DocumentationPromptFile, state)

	state, err = writeManagedFile(paths.CommitMessagePromptFile, renderCommitMessagePromptTemplate(), opts.Force)
	if err != nil {
		return scaffoldResult{}, err
	}
	recordScaffoldState(&result, paths.CommitMessagePromptFile, state)

	state, err = writeManagedFile(paths.DocumentationRulesFile, renderDocumentationInstructionTemplate(), opts.Force)
	if err != nil {
		return scaffoldResult{}, err
	}
	recordScaffoldState(&result, paths.DocumentationRulesFile, state)

	return result, nil
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
