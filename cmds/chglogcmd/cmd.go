package chglogcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/redant"
)

// ChangelogEntry represents a single changelog entry
type ChangelogEntry struct {
	Hash      string
	Date      time.Time
	Author    string
	Subject   string
	Body      string
	Type      string
	Scope     string
	Breaking  bool
	Refs      []string
}

// ChangelogSection represents a section of the changelog
type ChangelogSection struct {
	Title string
	Items []ChangelogEntry
}

// Changelog represents the entire changelog
type Changelog struct {
	Version   string
	Date      time.Time
	Sections []ChangelogSection
	Commits   []ChangelogEntry
}

// NewCommand creates the changelog command
func NewCommand() *redant.Command {
	var fromRef, toRef, outputFile string
	var includeBreaking, includeRefs bool

	app := &redant.Command{
		Use:   "changelog",
		Short: "Generate changelog between two refs (branches/tags)",
		Long:  `Generate a changelog between two Git refs (branches or tags) and output to a file or stdout`,
		Options: []redant.Option{
			{
				Flag:        "from",
				Description: "Source ref (branch/tag) to compare from (required)",
				Value:       redant.StringOf(&fromRef),
			},
			{
				Flag:        "to",
				Description: "Target ref (branch/tag) to compare to (required)",
				Value:       redant.StringOf(&toRef),
			},
			{
				Flag:        "output",
				Description: "Output file path (default: changelog.md)",
				Value:       redant.StringOf(&outputFile),
			},
			{
				Flag:        "breaking",
				Description: "Include breaking change indicators",
				Value:       redant.BoolOf(&includeBreaking),
			},
			{
				Flag:        "refs",
				Description: "Include commit references in changelog",
				Value:       redant.BoolOf(&includeRefs),
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) error {
			// Set defaults
			if outputFile == "" {
				outputFile = "changelog.md"
			}

			// Validate inputs
			if fromRef == "" || toRef == "" {
				log.Error().Msg("Both --from and --to refs must be specified")
				return nil
			}

			// Check if refs exist
			if !refExists(ctx, fromRef) {
				log.Error().Str("ref", fromRef).Msg("Source ref does not exist")
				return nil
			}
			if !refExists(ctx, toRef) {
				log.Error().Str("ref", toRef).Msg("Target ref does not exist")
				return nil
			}

			// Generate changelog
			changelog, err := generateChangelog(ctx, fromRef, toRef, includeBreaking, includeRefs)
			if err != nil {
				log.Err(err).Msg("Failed to generate changelog")
				return err
			}

			// Format and output changelog
			content := formatChangelog(changelog, fromRef, toRef)

			if outputFile != "stdout" && outputFile != "" {
				err = os.WriteFile(outputFile, []byte(content), 0644)
				if err != nil {
					log.Err(err).Str("file", outputFile).Msg("Failed to write changelog file")
					return err
				}
				fmt.Printf("Changelog written to %s\n", outputFile)
			} else {
				fmt.Println(content)
			}
			return nil
		},
	}

	return app
}

// refExists checks if a Git ref exists
func refExists(ctx context.Context, ref string) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", ref)
	err := cmd.Run()
	return err == nil
}

// generateChangelog generates changelog between two refs
func generateChangelog(ctx context.Context, fromRef, toRef string, includeBreaking, includeRefs bool) (*Changelog, error) {
	// Get commit differences between refs
	commits, err := getCommitsBetweenRefs(ctx, fromRef, toRef)
	if err != nil {
		return nil, err
	}

	// Parse commit messages to extract conventional commit information
	var changelogEntries []ChangelogEntry
	for _, commit := range commits {
		entry := parseCommitMessage(commit)
		changelogEntries = append(changelogEntries, entry)
	}

	// Organize commits into sections
	sections := organizeCommitsByType(changelogEntries)

	return &Changelog{
		Version:   fmt.Sprintf("%s...%s", fromRef, toRef),
		Date:      time.Now(),
		Sections: sections,
		Commits:   changelogEntries,
	}, nil
}

// getCommitsBetweenRefs gets commits between two refs
func getCommitsBetweenRefs(ctx context.Context, fromRef, toRef string) ([]string, error) {
	// Use git log to get commits between refs in reverse chronological order
	cmd := exec.CommandContext(ctx, "git", "log", "--reverse", "--pretty=format:%H|%ad|%an|%s|%b", "--date=iso", fmt.Sprintf("%s..%s", fromRef, toRef))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	commits := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, commit := range commits {
		if commit != "" {
			result = append(result, commit)
		}
	}

	return result, nil
}

// parseCommitMessage parses a commit message according to conventional commits specification
func parseCommitMessage(commitLine string) ChangelogEntry {
	parts := strings.SplitN(commitLine, "|", 5)
	if len(parts) < 4 {
		return ChangelogEntry{
			Hash:    "unknown",
			Subject: commitLine,
			Type:    "other",
		}
	}

	hash := parts[0]
	dateStr := parts[1]
	author := parts[2]
	subject := parts[3]
	body := ""
	if len(parts) > 4 {
		body = parts[4]
	}

	// Parse date
	date, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
	if err != nil {
		date = time.Now()
	}

	// Parse conventional commit format (type(scope): subject)
	var commitType, scope string
	var isBreaking bool
	var refs []string

	re := regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?!?:\s*(.*)$`)
	matches := re.FindStringSubmatch(subject)
	
	if len(matches) > 0 {
		commitType = matches[1]
		scope = matches[2]
		subject = matches[3]
		
		// Check if this is a breaking change (has ! in the header)
		if strings.Contains(matches[0], "!") {
			isBreaking = true
		}
	} else {
		// Default to "other" type if not conventional
		commitType = "other"
	}

	// Look for breaking changes in body
	if strings.Contains(strings.ToLower(body), "breaking change") || 
	   strings.Contains(strings.ToLower(body), "breaking-change") {
		isBreaking = true
	}

	// Extract issue references
	refs = extractRefs(subject + " " + body)

	return ChangelogEntry{
		Hash:     hash,
		Date:     date,
		Author:   author,
		Subject:  subject,
		Body:     body,
		Type:     commitType,
		Scope:    scope,
		Breaking: isBreaking,
		Refs:     refs,
	}
}

// extractRefs extracts issue references from commit message
func extractRefs(message string) []string {
	re := regexp.MustCompile(`#(\d+)`)
	matches := re.FindAllStringSubmatch(message, -1)
	
	var refs []string
	for _, match := range matches {
		if len(match) > 1 {
			refs = append(refs, match[1])
		}
	}
	
	return refs
}

// organizeCommitsByType organizes commits by type for changelog sections
func organizeCommitsByType(commits []ChangelogEntry) []ChangelogSection {
	sectionsMap := make(map[string][]ChangelogEntry)
	
	// Define section order
	sectionOrder := []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert", "other"}
	
	for _, commit := range commits {
		sectionType := commit.Type
		if sectionType == "" {
			sectionType = "other"
		}
		
		sectionsMap[sectionType] = append(sectionsMap[sectionType], commit)
	}
	
	var sections []ChangelogSection
	for _, sectionType := range sectionOrder {
		if commits, exists := sectionsMap[sectionType]; exists && len(commits) > 0 {
			title := getSectionTitle(sectionType)
			sections = append(sections, ChangelogSection{
				Title: title,
				Items: commits,
			})
		}
	}
	
	// Add any remaining types not in the predefined order
	for _, commit := range commits {
		alreadyAdded := false
		for _, section := range sections {
			if strings.EqualFold(section.Title, getSectionTitle(commit.Type)) {
				alreadyAdded = true
				break
			}
		}
		if !alreadyAdded {
			title := getSectionTitle(commit.Type)
			var items []ChangelogEntry
			for _, c := range commits {
				if c.Type == commit.Type {
					items = append(items, c)
				}
			}
			sections = append(sections, ChangelogSection{
				Title: title,
				Items: items,
			})
		}
	}
	
	return sections
}

// getSectionTitle gets the human-readable title for a commit type
func getSectionTitle(commitType string) string {
	switch strings.ToLower(commitType) {
	case "feat":
		return "Features"
	case "fix":
		return "Bug Fixes"
	case "docs":
		return "Documentation"
	case "style":
		return "Styles"
	case "refactor":
		return "Code Refactoring"
	case "perf":
		return "Performance Improvements"
	case "test":
		return "Tests"
	case "build":
		return "Build System"
	case "ci":
		return "Continuous Integration"
	case "chore":
		return "Chores"
	case "revert":
		return "Reverts"
	default:
		return strings.ToTitle(commitType)
	}
}

// formatChangelog formats the changelog as markdown
func formatChangelog(changelog *Changelog, fromRef, toRef string) string {
	var result strings.Builder
	
	result.WriteString(fmt.Sprintf("# Changelog\n\n"))
	result.WriteString(fmt.Sprintf("Changelog from `%s` to `%s` (Generated on %s)\n\n", fromRef, toRef, changelog.Date.Format("2006-01-02")))
	
	for _, section := range changelog.Sections {
		result.WriteString(fmt.Sprintf("## %s\n\n", section.Title))
		
		for _, item := range section.Items {
			line := fmt.Sprintf("- %s", item.Subject)
			
			if item.Breaking {
				line = fmt.Sprintf("⚠️  %s", line)
			}
			
			if item.Scope != "" {
				line = fmt.Sprintf("- **%s**: %s", item.Scope, item.Subject)
			}
			
			result.WriteString(line)
			
			if len(item.Refs) > 0 {
				var refLinks []string
				for _, ref := range item.Refs {
					refLinks = append(refLinks, fmt.Sprintf("#%s", ref))
				}
				result.WriteString(fmt.Sprintf(" (%s)", strings.Join(refLinks, ", ")))
			}
			
			result.WriteString(fmt.Sprintf(" ([%s](%s))\n", item.Hash[:7], getCommitURL(item.Hash)))
		}
		
		result.WriteString("\n")
	}
	
	return result.String()
}

// getCommitURL gets the URL for a commit (this would need to be customized based on the repository host)
func getCommitURL(hash string) string {
	// This is a placeholder - in a real implementation you might detect the repo host (GitHub, GitLab, etc.)
	// and construct the appropriate URL
	return fmt.Sprintf("https://github.com/unknown/unknown/commit/%s", hash)
}