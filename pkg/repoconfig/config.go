package repoconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const repoConfigDir = ".fastgit"

// Policy defines team governance rules for a repository.
type Policy struct {
	Enforce bool `yaml:"enforce"`
	Branch  struct {
		Pattern string `yaml:"pattern"`
	} `yaml:"branch"`
	ProtectedBranches []string `yaml:"protected_branches"`
	Commit            struct {
		Conventional bool `yaml:"conventional"`
	} `yaml:"commit"`
	SensitivePaths []string `yaml:"sensitive_paths"`
}

// CommitSettings defines AI commit generation preferences.
type CommitSettings struct {
	Locale            string   `yaml:"locale"`
	MaxLength         int      `yaml:"max_length"`
	RequireScope      bool     `yaml:"require_scope"`
	CandidatesDefault bool     `yaml:"candidates_default"`
	Types             []string `yaml:"types"`
}

// Bundle contains repository-local fastgit settings.
type Bundle struct {
	RepoRoot string
	Policy   Policy
	Commit   CommitSettings
}

// Load reads `.fastgit/policy.yaml` and `.fastgit/commit.yaml` when present.
func Load(repoRoot string) (Bundle, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return Bundle{}, fmt.Errorf("repo root is empty")
	}

	bundle := Bundle{
		RepoRoot: repoRoot,
		Commit: CommitSettings{
			Locale:    "en",
			MaxLength: 72,
			Types:     []string{"feat", "fix", "chore", "docs", "refactor", "test", "build", "ci"},
		},
	}

	policyPath := filepath.Join(repoRoot, repoConfigDir, "policy.yaml")
	if err := readYAML(policyPath, &bundle.Policy); err != nil {
		return Bundle{}, err
	}

	commitPath := filepath.Join(repoRoot, repoConfigDir, "commit.yaml")
	if err := readYAML(commitPath, &bundle.Commit); err != nil {
		return Bundle{}, err
	}

	if bundle.Commit.MaxLength <= 0 {
		bundle.Commit.MaxLength = 72
	}
	if strings.TrimSpace(bundle.Commit.Locale) == "" {
		bundle.Commit.Locale = "en"
	}
	return bundle, nil
}

// InitScaffold writes default team config files if missing.
func InitScaffold(repoRoot string) ([]string, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return nil, fmt.Errorf("repo root is empty")
	}
	dir := filepath.Join(repoRoot, repoConfigDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	created := make([]string, 0, 2)
	policyPath := filepath.Join(dir, "policy.yaml")
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		if err := os.WriteFile(policyPath, []byte(defaultPolicyYAML), 0o644); err != nil {
			return nil, err
		}
		created = append(created, policyPath)
	}

	commitPath := filepath.Join(dir, "commit.yaml")
	if _, err := os.Stat(commitPath); os.IsNotExist(err) {
		if err := os.WriteFile(commitPath, []byte(defaultCommitYAML), 0o644); err != nil {
			return nil, err
		}
		created = append(created, commitPath)
	}
	return created, nil
}

// ValidateBranch checks the current branch against policy.
func (b Bundle) ValidateBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	pattern := strings.TrimSpace(b.Policy.Branch.Pattern)
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid branch.pattern: %w", err)
	}
	if !re.MatchString(branch) {
		return fmt.Errorf("branch %q does not match pattern %q", branch, pattern)
	}
	return nil
}

// IsProtectedBranch reports whether direct pushes should be blocked.
func (b Bundle) IsProtectedBranch(branch string) bool {
	branch = strings.TrimSpace(branch)
	for _, protected := range b.Policy.ProtectedBranches {
		if branch == strings.TrimSpace(protected) {
			return true
		}
	}
	return false
}

// MatchesSensitivePath reports whether a path should trigger extra review.
func (b Bundle) MatchesSensitivePath(path string) bool {
	path = filepath.ToSlash(strings.TrimSpace(path))
	for _, pattern := range b.Policy.SensitivePaths {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if strings.Contains(path, strings.Trim(pattern, "*")) {
			return true
		}
	}
	return false
}

// CheckBranch returns a blocking error when enforce is on and the branch violates policy.
func (b Bundle) CheckBranch(branch string, skipPolicy bool) error {
	err := b.ValidateBranch(branch)
	if err == nil || skipPolicy || !b.Policy.Enforce {
		return nil
	}
	return fmt.Errorf("branch blocked by .fastgit/policy.yaml: %w (use --skip-policy to bypass)", err)
}

// CheckCommitMessage returns a blocking error when enforce is on and the message violates policy.
func (b Bundle) CheckCommitMessage(message string, skipPolicy bool) error {
	err := b.ValidateCommitMessage(message)
	if err == nil || skipPolicy || !b.Policy.Enforce {
		return nil
	}
	return fmt.Errorf("commit message blocked by .fastgit/policy.yaml: %w (use --skip-policy to bypass)", err)
}

// WarnCommitMessage returns validation issues when enforce is off.
func (b Bundle) WarnCommitMessage(message string) error {
	if b.Policy.Enforce {
		return nil
	}
	return b.ValidateCommitMessage(message)
}

// ValidatePush blocks direct pushes to protected branches unless override is set.
func (b Bundle) ValidatePush(branch string, override bool) error {
	if override {
		return nil
	}
	branch = strings.TrimSpace(branch)
	if b.IsProtectedBranch(branch) {
		return fmt.Errorf(
			"push to protected branch %q is blocked by .fastgit/policy.yaml; use a feature branch and open a PR (or pass --override-policy)",
			branch,
		)
	}
	return nil
}

// ValidateCommitMessage checks a message against policy and commit settings.
func (b Bundle) ValidateCommitMessage(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("commit message is empty")
	}
	if len(message) > b.Commit.MaxLength {
		return fmt.Errorf("commit message exceeds max_length %d", b.Commit.MaxLength)
	}
	if !b.Policy.Commit.Conventional {
		return nil
	}
	if !regexp.MustCompile(`^[a-z]+(\([^)]+\))?!?:\s+.+`).MatchString(message) {
		return fmt.Errorf("commit message must follow conventional format (type(scope)?: subject)")
	}
	if b.Commit.RequireScope && !strings.Contains(message, "(") {
		return fmt.Errorf("commit scope is required by .fastgit/commit.yaml")
	}
	return nil
}

func readYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	return yaml.Unmarshal(data, target)
}

const defaultPolicyYAML = `enforce: true

branch:
  pattern: "^(feature|fix|chore|docs)/[a-z0-9._/-]+$"

protected_branches:
  - main
  - master

commit:
  conventional: true

sensitive_paths:
  - ".env"
  - ".env.*"
  - "**/*credentials*"
  - "**/*secret*"
`

const defaultCommitYAML = `locale: en
max_length: 72
require_scope: false
candidates_default: true
types:
  - feat
  - fix
  - chore
  - docs
  - refactor
  - test
  - build
  - ci
`
