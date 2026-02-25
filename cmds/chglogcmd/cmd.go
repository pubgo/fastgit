package chglogcmd

import (
	"context"
	"fmt"
	"net/url"
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
	Hash        string
	Date        time.Time
	Author      string
	AuthorEmail string
	Subject     string
	Body        string
	Type        string
	Scope       string
	Breaking    bool
	Refs        []string
	IsPRMerge   bool
	PRNumber    string
	PRTitle     string
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
	StartDate time.Time
	EndDate   time.Time
	Sections  []ChangelogSection
	Commits   []ChangelogEntry
	CommitURL string
	IssueURL  string
	PRURL     string
}

// CommitRecord represents raw commit fields from git log
type CommitRecord struct {
	Hash        string
	Date        string
	Author      string
	AuthorEmail string
	Subject     string
	Body        string
}

// NewCommand creates the changelog command
func NewCommand() *redant.Command {
	var fromRef, toRef, outputFile string
	var includeBreaking, includeRefs bool
	includeAuthor := true
	var style string
	var keepExtra bool

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
			{
				Flag:        "author",
				Description: "Include authors section in changelog",
				Value:       redant.BoolOf(&includeAuthor),
			},
			{
				Flag:        "style",
				Description: "Changelog grouping style: conventional|keepachangelog (default: conventional)",
				Value:       redant.StringOf(&style),
			},
			{
				Flag:        "keep-extra",
				Description: "Keep extra sections when using keepachangelog (e.g., Chores/Build/CI/Tests)",
				Value:       redant.BoolOf(&keepExtra),
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) error {
			// Set defaults
			if outputFile == "" {
				outputFile = "changelog.md"
			}
			if style == "" {
				style = "conventional"
			}

			// Auto resolve refs if not provided
			if toRef == "" {
				toRef = "HEAD"
				log.Info().Str("ref", toRef).Msg("Auto-set target ref")
			}
			if fromRef == "" {
				fromRef = getLastTag(ctx)
				if fromRef == "" {
					fromRef = getRootCommit(ctx)
				}
				if fromRef != "" {
					log.Info().Str("ref", fromRef).Msg("Auto-set source ref")
				}
			}

			// Validate inputs
			if fromRef == "" || toRef == "" {
				log.Error().Msg("Both --from and --to refs must be specified or auto-detected")
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
			changelog, err := generateChangelog(ctx, fromRef, toRef, style, keepExtra)
			if err != nil {
				log.Err(err).Msg("Failed to generate changelog")
				return err
			}

			// Format and output changelog
			content := formatChangelog(changelog, fromRef, toRef, includeBreaking, includeRefs, includeAuthor)

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
func generateChangelog(ctx context.Context, fromRef, toRef, style string, keepExtra bool) (*Changelog, error) {
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
	sections := organizeCommitsByStyle(changelogEntries, style, keepExtra)
	startDate, endDate := computeRangeDates(changelogEntries)

	commitURL, issueURL, prURL := detectRepoURLs(ctx)

	return &Changelog{
		Version:   fmt.Sprintf("%s...%s", fromRef, toRef),
		Date:      time.Now(),
		StartDate: startDate,
		EndDate:   endDate,
		Sections:  sections,
		Commits:   changelogEntries,
		CommitURL: commitURL,
		IssueURL:  issueURL,
		PRURL:     prURL,
	}, nil
}

// getCommitsBetweenRefs gets commits between two refs
func getCommitsBetweenRefs(ctx context.Context, fromRef, toRef string) ([]CommitRecord, error) {
	// Use git log to get commits between refs in reverse chronological order
	// Use non-printable separators to avoid conflicts with subject/body content
	format := "%H%x1f%ad%x1f%an%x1f%ae%x1f%s%x1f%b%x1e"
	cmd := exec.CommandContext(ctx, "git", "log", "--reverse", "--pretty=format:"+format, "--date=iso", fmt.Sprintf("%s..%s", fromRef, toRef))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result []CommitRecord
	records := strings.Split(string(output), "\x1e")
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		fields := strings.Split(record, "\x1f")
		if len(fields) < 5 {
			continue
		}
		rec := CommitRecord{
			Hash:        strings.TrimSpace(fields[0]),
			Date:        strings.TrimSpace(fields[1]),
			Author:      strings.TrimSpace(fields[2]),
			AuthorEmail: strings.TrimSpace(fields[3]),
			Subject:     strings.TrimSpace(fields[4]),
		}
		if len(fields) > 5 {
			rec.Body = strings.TrimSpace(strings.Join(fields[5:], "\x1f"))
		}
		result = append(result, rec)
	}

	return result, nil
}

// parseCommitMessage parses a commit message according to conventional commits specification
func parseCommitMessage(commit CommitRecord) ChangelogEntry {
	hash := commit.Hash
	dateStr := commit.Date
	author := commit.Author
	authorEmail := commit.AuthorEmail
	subject := commit.Subject
	body := commit.Body

	// Parse date
	date, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
	if err != nil {
		date = time.Now()
	}

	// Detect PR merge or squash patterns
	prNumber, prTitle, isPR := detectPR(subject, body)

	// Parse conventional commit format (type(scope): subject)
	var commitType, scope string
	var isBreaking bool
	var refs []string

	re := regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9_/-]*)(?:\(([^)]+)\))?(!)?:\s*(.*)$`)
	matches := re.FindStringSubmatch(subject)

	if len(matches) > 0 {
		commitType = strings.ToLower(matches[1])
		scope = matches[2]
		subject = matches[4]

		// Check if this is a breaking change (has ! in the header)
		if matches[3] == "!" {
			isBreaking = true
		}
	} else {
		// Heuristic fallback for non-conventional commits
		commitType = classifyFallback(subject, body)
	}

	// Look for breaking changes in body
	if strings.Contains(strings.ToLower(body), "breaking change") ||
		strings.Contains(strings.ToLower(body), "breaking-change") {
		isBreaking = true
	}

	// Extract issue references
	refs = extractRefs(subject + " " + body)

	return ChangelogEntry{
		Hash:        hash,
		Date:        date,
		Author:      author,
		AuthorEmail: authorEmail,
		Subject:     subject,
		Body:        body,
		Type:        commitType,
		Scope:       scope,
		Breaking:    isBreaking,
		Refs:        refs,
		IsPRMerge:   isPR,
		PRNumber:    prNumber,
		PRTitle:     prTitle,
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
	var prItems []ChangelogEntry

	// Define section order
	sectionOrder := []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert", "other"}

	for _, commit := range commits {
		if commit.IsPRMerge {
			prItems = append(prItems, commit)
			continue
		}
		sectionType := commit.Type
		if sectionType == "" {
			sectionType = "other"
		}
		sectionsMap[sectionType] = append(sectionsMap[sectionType], commit)
	}

	var sections []ChangelogSection
	if len(prItems) > 0 {
		sections = append(sections, ChangelogSection{
			Title: "Merged Pull Requests",
			Items: prItems,
		})
	}

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
		if commit.IsPRMerge {
			continue
		}
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
				if c.Type == commit.Type && !c.IsPRMerge {
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
func formatChangelog(changelog *Changelog, fromRef, toRef string, includeBreaking, includeRefs, includeAuthor bool) string {
	var result strings.Builder

	result.WriteString("# Changelog\n\n")
	result.WriteString(fmt.Sprintf("Changelog from `%s` (%s) to `%s` (%s) (Generated on %s)\n\n",
		fromRef,
		formatRangeTime(changelog.StartDate),
		toRef,
		formatRangeTime(changelog.EndDate),
		changelog.Date.Format("2006-01-02"),
	))

	for _, section := range changelog.Sections {
		result.WriteString(fmt.Sprintf("## %s\n\n", section.Title))

		for _, item := range section.Items {
			linePrefix := "- "
			if includeBreaking && item.Breaking {
				linePrefix += "⚠️ "
			}

			lineBody := buildEntryTitle(item, changelog.PRURL)
			result.WriteString(linePrefix + lineBody)

			if includeRefs && len(item.Refs) > 0 {
				var refLinks []string
				for _, ref := range item.Refs {
					refLinks = append(refLinks, formatIssueLink(ref, changelog.IssueURL))
				}
				result.WriteString(fmt.Sprintf(" (%s)", strings.Join(refLinks, ", ")))
			}

			result.WriteString(fmt.Sprintf(" (%s)\n", formatCommitLink(item.Hash, changelog.CommitURL)))
		}

		result.WriteString("\n")
	}

	if includeAuthor {
		authors := collectAuthors(changelog.Commits)
		if len(authors) > 0 {
			result.WriteString("## Authors\n\n")
			for _, author := range authors {
				if author.Email != "" {
					result.WriteString(fmt.Sprintf("- %s <%s>\n", author.Name, author.Email))
				} else {
					result.WriteString(fmt.Sprintf("- %s\n", author.Name))
				}
			}
			result.WriteString("\n")
		}
	}

	return result.String()
}

func buildEntryTitle(item ChangelogEntry, prURL string) string {
	if item.IsPRMerge {
		title := strings.TrimSpace(item.PRTitle)
		if title == "" {
			title = strings.TrimSpace(item.Subject)
		}
		if item.PRNumber != "" {
			prDisplay := fmt.Sprintf("#%s", item.PRNumber)
			if prURL != "" {
				prDisplay = fmt.Sprintf("[#%s](%s)", item.PRNumber, fmt.Sprintf(prURL, item.PRNumber))
			}
			if title != "" {
				return fmt.Sprintf("%s %s", prDisplay, title)
			}
			return prDisplay
		}
		if title != "" {
			return title
		}
		return "(unnamed pull request)"
	}

	if item.Scope != "" {
		return fmt.Sprintf("**%s**: %s", item.Scope, item.Subject)
	}
	return item.Subject
}

func formatCommitLink(hash, commitURL string) string {
	short := shortHash(hash)
	if commitURL == "" || hash == "" {
		return short
	}
	return fmt.Sprintf("[%s](%s)", short, fmt.Sprintf(commitURL, hash))
}

func formatIssueLink(ref, issueURL string) string {
	if issueURL == "" {
		return fmt.Sprintf("#%s", ref)
	}
	return fmt.Sprintf("[#%s](%s)", ref, fmt.Sprintf(issueURL, ref))
}

func shortHash(hash string) string {
	if len(hash) >= 7 {
		return hash[:7]
	}
	if hash == "" {
		return "unknown"
	}
	return hash
}

func detectPR(subject, body string) (string, string, bool) {
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)

	mergeRe := regexp.MustCompile(`(?i)^merge (pull request|pr) #(\d+)`)
	if matches := mergeRe.FindStringSubmatch(subject); len(matches) > 0 {
		title := firstNonEmptyLine(body)
		return matches[2], title, true
	}

	squashRe := regexp.MustCompile(`\(#(\d+)\)\s*$`)
	if matches := squashRe.FindStringSubmatch(subject); len(matches) > 0 {
		title := strings.TrimSpace(squashRe.ReplaceAllString(subject, ""))
		return matches[1], title, true
	}

	prRefRe := regexp.MustCompile(`(?i)\b(?:pull request|pr)\s*#(\d+)\b`)
	if matches := prRefRe.FindStringSubmatch(subject + " " + body); len(matches) > 0 {
		title := strings.TrimSpace(subject)
		return matches[1], title, true
	}

	return "", "", false
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func classifyFallback(subject, body string) string {
	text := strings.ToLower(subject + " " + body)
	switch {
	case containsAny(text, "fix", "bug", "hotfix", "patch"):
		return "fix"
	case containsAny(text, "feat", "feature", "add", "新增", "增加"):
		return "feat"
	case containsAny(text, "security", "cve", "vuln"):
		return "security"
	case containsAny(text, "deprecate", "deprecated"):
		return "deprecated"
	case containsAny(text, "doc", "docs", "readme", "文档"):
		return "docs"
	case containsAny(text, "refactor", "clean", "重构"):
		return "refactor"
	case containsAny(text, "perf", "opt", "optimize", "performance", "性能"):
		return "perf"
	case containsAny(text, "test", "tests", "unit", "e2e"):
		return "test"
	case containsAny(text, "build", "deps", "dependency"):
		return "build"
	case containsAny(text, "ci", "pipeline", "workflow"):
		return "ci"
	case containsAny(text, "chore", "misc"):
		return "chore"
	case containsAny(text, "style", "format", "lint"):
		return "style"
	case containsAny(text, "revert", "回滚"):
		return "revert"
	default:
		return "other"
	}
}

func organizeCommitsByStyle(commits []ChangelogEntry, style string, keepExtra bool) []ChangelogSection {
	style = strings.ToLower(strings.TrimSpace(style))
	switch style {
	case "keepachangelog", "keep-a-changelog", "keep":
		return organizeCommitsKeepAChangelog(commits, keepExtra)
	case "conventional", "default", "":
		return organizeCommitsByType(commits)
	default:
		log.Warn().Str("style", style).Msg("Unknown changelog style, falling back to conventional")
		return organizeCommitsByType(commits)
	}
}

func organizeCommitsKeepAChangelog(commits []ChangelogEntry, keepExtra bool) []ChangelogSection {
	sectionsMap := make(map[string][]ChangelogEntry)
	var prItems []ChangelogEntry

	sectionOrder := buildKeepAChangelogOrder(keepExtra)

	for _, commit := range commits {
		if commit.IsPRMerge {
			prItems = append(prItems, commit)
			continue
		}
		section := mapTypeToKeepSection(commit, keepExtra)
		sectionsMap[section] = append(sectionsMap[section], commit)
	}

	var sections []ChangelogSection
	if len(prItems) > 0 {
		sections = append(sections, ChangelogSection{
			Title: "Merged Pull Requests",
			Items: prItems,
		})
	}

	for _, section := range sectionOrder {
		if section == "Merged Pull Requests" {
			continue
		}
		if items, exists := sectionsMap[section]; exists && len(items) > 0 {
			sections = append(sections, ChangelogSection{
				Title: section,
				Items: items,
			})
		}
	}

	return sections
}

func mapTypeToKeepSection(commit ChangelogEntry, keepExtra bool) string {
	switch strings.ToLower(commit.Type) {
	case "feat":
		return "Added"
	case "fix":
		return "Fixed"
	case "docs":
		return "Documentation"
	case "deprecated", "deprecate":
		return "Deprecated"
	case "revert", "remove", "removed":
		return "Removed"
	case "security":
		return "Security"
	case "refactor":
		if keepExtra {
			return "Code Refactoring"
		}
		return "Changed"
	case "perf":
		if keepExtra {
			return "Performance Improvements"
		}
		return "Changed"
	case "style":
		if keepExtra {
			return "Styles"
		}
		return "Changed"
	case "build":
		if keepExtra {
			return "Build System"
		}
		return "Changed"
	case "ci":
		if keepExtra {
			return "Continuous Integration"
		}
		return "Changed"
	case "test":
		if keepExtra {
			return "Tests"
		}
		return "Changed"
	case "chore":
		if keepExtra {
			return "Chores"
		}
		return "Changed"
	default:
		return "Other"
	}
}

func buildKeepAChangelogOrder(keepExtra bool) []string {
	base := []string{
		"Merged Pull Requests",
		"Added",
		"Changed",
		"Fixed",
		"Deprecated",
		"Removed",
		"Security",
		"Documentation",
	}
	if keepExtra {
		return append(base, []string{
			"Code Refactoring",
			"Performance Improvements",
			"Styles",
			"Build System",
			"Continuous Integration",
			"Tests",
			"Chores",
			"Other",
		}...)
	}
	return append(base, "Other")
}

type AuthorInfo struct {
	Name  string
	Email string
}

func collectAuthors(entries []ChangelogEntry) []AuthorInfo {
	seen := make(map[string]struct{})
	var authors []AuthorInfo
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Author)
		email := strings.TrimSpace(entry.AuthorEmail)
		if name == "" && email == "" {
			continue
		}
		key := strings.ToLower(name + "|" + email)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if name == "" {
			name = email
		}
		authors = append(authors, AuthorInfo{Name: name, Email: email})
	}
	return authors
}

func computeRangeDates(entries []ChangelogEntry) (time.Time, time.Time) {
	if len(entries) == 0 {
		now := time.Now()
		return now, now
	}
	start := entries[0].Date
	end := entries[0].Date
	for _, entry := range entries {
		if entry.Date.Before(start) {
			start = entry.Date
		}
		if entry.Date.After(end) {
			end = entry.Date
		}
	}
	return start, end
}

func formatRangeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("2006-01-02 15:04:05")
}

func getLastTag(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "git", "describe", "--tags", "--abbrev=0")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func getRootCommit(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "git", "rev-list", "--max-parents=0", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func detectRepoURLs(ctx context.Context) (string, string, string) {
	remote := getGitRemote(ctx)
	if remote == "" {
		return "", "", ""
	}

	host, owner, repo := parseRemoteURL(remote)
	if host == "" || owner == "" || repo == "" {
		return "", "", ""
	}

	if strings.Contains(strings.ToLower(host), "gitlab") {
		return fmt.Sprintf("https://%s/%s/%s/-/commit/%%s", host, owner, repo),
			fmt.Sprintf("https://%s/%s/%s/-/issues/%%s", host, owner, repo),
			fmt.Sprintf("https://%s/%s/%s/-/merge_requests/%%s", host, owner, repo)
	}

	return fmt.Sprintf("https://%s/%s/%s/commit/%%s", host, owner, repo),
		fmt.Sprintf("https://%s/%s/%s/issues/%%s", host, owner, repo),
		fmt.Sprintf("https://%s/%s/%s/pull/%%s", host, owner, repo)
}

func getGitRemote(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func parseRemoteURL(remote string) (string, string, string) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", "", ""
	}

	sshRe := regexp.MustCompile(`^git@([^:]+):(.+)$`)
	if matches := sshRe.FindStringSubmatch(remote); len(matches) > 0 {
		host := matches[1]
		path := strings.TrimPrefix(matches[2], "/")
		owner, repo := splitOwnerRepo(path)
		return host, owner, repo
	}

	parsed, err := url.Parse(remote)
	if err != nil {
		return "", "", ""
	}
	path := strings.Trim(parsed.Path, "/")
	owner, repo := splitOwnerRepo(path)
	return parsed.Hostname(), owner, repo
}

func splitOwnerRepo(path string) (string, string) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", ""
	}
	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")
	return owner, repo
}
