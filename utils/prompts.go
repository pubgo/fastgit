package utils

import (
	"fmt"
	"strings"
)

type CommitType string

const (
	EmptyCommitType        CommitType = ""
	ConventionalCommitType CommitType = "conventional"
)

const conventionalData = `
Choose a type from the type-to-description JSON below that best describes the git diff:
{
  "docs": "Documentation only changes",
  "style": "Changes that do not affect the meaning of the code (white-space, formatting, missing semi-colons, etc)",
  "refactor": "A code change that neither fixes a bug nor adds a feature",
  "perf": "A code change that improves performance",
  "test": "Adding missing tests or correcting existing tests",
  "build": "Changes that affect the build system or external dependencies",
  "ci": "Changes to our CI configuration files and scripts",
  "chore": "Other changes that don't modify src or test files",
  "revert": "Reverts a previous commit",
  "feat": "A new feature",
  "fix": "A bug fix"
}`

var commitTypeFormats = map[CommitType]string{
	EmptyCommitType:        "<commit message>",
	ConventionalCommitType: "<type>(<optional scope>): <commit message>",
}

func specifyCommitFormat(commitType CommitType) string {
	return fmt.Sprintf("The output response must be in format:\n%s", commitTypeFormats[commitType])
}

var commitTypes = map[CommitType]string{
	ConventionalCommitType: conventionalData,
	EmptyCommitType:        "",
}

func GeneratePrompt(locale string, maxLength int, commitType CommitType) string {
	promptParts := []string{
		"Generate a concise git commit message written in present tense for the following code diff with the given specifications below:",
		fmt.Sprintf("Message language: %s", locale),
		fmt.Sprintf("Commit message must be a maximum of %d characters.", maxLength),
		"Exclude anything unnecessary such as translation. Your entire response will be passed directly into git commit.",
		commitTypes[commitType],
		specifyCommitFormat(commitType),
	}

	// Filter out empty strings
	var filteredParts []string
	for _, part := range promptParts {
		if part != "" {
			filteredParts = append(filteredParts, part)
		}
	}

	return strings.Join(filteredParts, "\n")
}

// AppendAllowedTypes adds team commit type constraints to a prompt.
func AppendAllowedTypes(prompt string, allowedTypes []string) string {
	if len(allowedTypes) == 0 {
		return prompt
	}
	return prompt + "\nPrefer commit types from this team list: " + strings.Join(allowedTypes, ", ")
}
