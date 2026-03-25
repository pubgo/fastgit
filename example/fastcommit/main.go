package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/pubgo/redant"
	agentlineapp "github.com/pubgo/redant/cmds/agentlineapp"
	"github.com/pubgo/redant/cmds/completioncmd"
	"github.com/pubgo/redant/cmds/mcpcmd"
	"github.com/pubgo/redant/cmds/readlinecmd"
	"github.com/pubgo/redant/cmds/richlinecmd"
	"github.com/pubgo/redant/cmds/webcmd"
	"github.com/pubgo/redant/cmds/webttycmd"
	agentlinemodule "github.com/pubgo/redant/pkg/agentline"
)

// mkdir -p ~/.zsh/completions
// go run example/fastcommit/main.go completion zsh > ~/.zsh/completions/_fastcommit

type CommitMetadata struct {
	Ticket   string            `json:"ticket" yaml:"ticket"`
	Priority string            `json:"priority" yaml:"priority"`
	Labels   []string          `json:"labels" yaml:"labels"`
	Extra    map[string]string `json:"extra" yaml:"extra"`
}

type ReleasePlan struct {
	Strategy string   `json:"strategy" yaml:"strategy"`
	Canary   int      `json:"canary" yaml:"canary"`
	Services []string `json:"services" yaml:"services"`
}

type RepoPolicy struct {
	ProtectedBranches []string `json:"protectedBranches" yaml:"protectedBranches"`
	RequireReview     bool     `json:"requireReview" yaml:"requireReview"`
	MinApprovals      int      `json:"minApprovals" yaml:"minApprovals"`
}

func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("<marshal-error: %v>", err)
	}
	return string(b)
}

func main() {
	rootCmd := &redant.Command{
		Use:   "fastcommit",
		Short: "A fast commit tool.",
		Long:  "A tool for making fast commits with rich command tree and complex option types.",
	}

	var (
		commitMessage    string
		commitAmend      bool
		commitFormat     string
		commitLabels     []string
		commitReviewers  []string
		commitTimeout    time.Duration
		commitWeight     float64
		commitMaxRetries int64
		commitWebhookURL url.URL
	)
	commitEndpoint := &redant.HostPort{}
	commitPattern := &redant.Regexp{}
	commitMetadata := &redant.Struct[CommitMetadata]{Value: CommitMetadata{
		Ticket:   "JIRA-100",
		Priority: "high",
		Labels:   []string{"feat", "backend"},
		Extra:    map[string]string{"source": "cli"},
	}}

	commitCmd := &redant.Command{
		Use:   "commit",
		Short: "Commit changes.",
		Long:  "Commit changes with advanced options and typed values.",
		Metadata: map[string]string{
			agentlinemodule.CommandMetaAgentCommand: "true",
		},
		Options: redant.OptionSet{
			{
				Flag:        "message",
				Shorthand:   "m",
				Description: "Commit message.",
				Value:       redant.StringOf(&commitMessage),
				Default:     "update: default message",
			},
			{
				Flag:        "amend",
				Description: "Amend the previous commit.",
				Value:       redant.BoolOf(&commitAmend),
			},
			{
				Flag:        "format",
				Description: "Output format.",
				Value:       redant.EnumOf(&commitFormat, "text", "json", "yaml"),
				Default:     "text",
			},
			{
				Flag:        "labels",
				Description: "Commit labels enum-array.",
				Value:       redant.EnumArrayOf(&commitLabels, "feat", "fix", "docs", "refactor", "test", "chore"),
			},
			{
				Flag:        "reviewers",
				Description: "Reviewers list.",
				Value:       redant.StringArrayOf(&commitReviewers),
			},
			{
				Flag:        "timeout",
				Description: "Commit timeout duration.",
				Value:       redant.DurationOf(&commitTimeout),
				Default:     "30s",
			},
			{
				Flag:        "weight",
				Description: "Commit score weight.",
				Value:       redant.Float64Of(&commitWeight),
				Default:     "1.5",
			},
			{
				Flag:        "max-retries",
				Description: "Max retry count.",
				Value:       redant.Int64Of(&commitMaxRetries),
				Default:     "3",
			},
			{
				Flag:        "webhook",
				Description: "Webhook URL.",
				Value:       redant.URLOf(&commitWebhookURL),
				Default:     "https://example.com/hook",
			},
			{
				Flag:        "endpoint",
				Description: "Commit target endpoint (host:port).",
				Value:       commitEndpoint,
				Default:     "127.0.0.1:9000",
			},
			{
				Flag:        "pattern",
				Description: "Filter regexp.",
				Value:       commitPattern,
				Default:     "^(feat|fix|docs):",
			},
			{
				Flag:        "metadata",
				Description: "Commit metadata as struct (JSON/YAML).",
				Value:       commitMetadata,
			},
		},
		Args: redant.ArgSet{
			{Name: "files", Description: "Files to commit (positional).", Value: redant.StringOf(new(string))},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			fmt.Printf("[commit] args=%v\n", inv.Args)
			fmt.Printf("[commit] message=%q amend=%v format=%s timeout=%s weight=%.2f max-retries=%d\n", commitMessage, commitAmend, commitFormat, commitTimeout, commitWeight, commitMaxRetries)
			fmt.Printf("[commit] labels=%v reviewers=%v endpoint=%s webhook=%s pattern=%s\n", commitLabels, commitReviewers, commitEndpoint.String(), redant.URLOf(&commitWebhookURL).String(), commitPattern.String())
			fmt.Printf("[commit] metadata=%s\n", toJSON(commitMetadata.Value))
			return nil
		},
	}

	var (
		detailedAuthor  string
		detailedVerbose bool
		detailedMode    string
	)

	detailedCmd := &redant.Command{
		Use:   "detailed",
		Short: "Detailed commit.",
		Long:  "Commit with detailed options.",
		Options: redant.OptionSet{
			{
				Flag:        "author",
				Description: "Author of the commit.",
				Value:       redant.StringOf(&detailedAuthor),
			},
			{
				Flag:        "verbose",
				Shorthand:   "v",
				Description: "Verbose output.",
				Value:       redant.BoolOf(&detailedVerbose),
			},
			{
				Flag:        "mode",
				Description: "Detailed mode.",
				Value:       redant.EnumOf(&detailedMode, "diff", "stat", "full"),
				Default:     "diff",
			},
		},
		Args: redant.ArgSet{
			{Name: "files", Description: "Files to commit.", Value: redant.StringOf(new(string))},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			fmt.Printf("[commit detailed] args=%v author=%q verbose=%v mode=%s\n", inv.Args, detailedAuthor, detailedVerbose, detailedMode)
			return nil
		},
	}

	var (
		releaseChannel   string
		releaseRegions   []string
		releaseBatchSize int64
		releaseWindow    time.Duration
		releaseDryRun    bool
		releaseVersion   string
	)
	releaseFilter := &redant.Regexp{}
	releasePlan := &redant.Struct[ReleasePlan]{Value: ReleasePlan{
		Strategy: "canary",
		Canary:   10,
		Services: []string{"api", "worker"},
	}}
	releaseShipCmd := &redant.Command{
		Use:   "release ship",
		Short: "Ship a release with rollout controls.",
		Long:  "Ship release with enum, enum-array, duration, struct, regexp and integer options.",
		Metadata: map[string]string{
			agentlinemodule.CommandMetaAgentCommand: "true",
		},
		Options: redant.OptionSet{
			{Flag: "channel", Description: "Release channel.", Value: redant.EnumOf(&releaseChannel, "alpha", "beta", "stable"), Default: "beta"},
			{Flag: "regions", Description: "Target regions.", Value: redant.EnumArrayOf(&releaseRegions, "cn", "us", "eu", "ap")},
			{Flag: "batch-size", Description: "Batch size.", Value: redant.Int64Of(&releaseBatchSize), Default: "100"},
			{Flag: "window", Description: "Release window.", Value: redant.DurationOf(&releaseWindow), Default: "5m"},
			{Flag: "dry-run", Description: "Preview only.", Value: redant.BoolOf(&releaseDryRun)},
			{Flag: "filter", Description: "Service name filter regexp.", Value: releaseFilter, Default: "^(api|worker)$"},
			{Flag: "plan", Description: "Rollout plan object.", Value: releasePlan},
		},
		Args: redant.ArgSet{
			{Name: "version", Required: true, Description: "Release version.", Value: redant.StringOf(&releaseVersion)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			fmt.Printf("[release ship] version=%s channel=%s dry-run=%v regions=%v batch-size=%d window=%s filter=%s\n", releaseVersion, releaseChannel, releaseDryRun, releaseRegions, releaseBatchSize, releaseWindow, releaseFilter.String())
			fmt.Printf("[release ship] plan=%s\n", toJSON(releasePlan.Value))
			return nil
		},
	}

	var (
		repoName       string
		repoVisibility string
		repoTags       []string
		repoMirrorURL  url.URL
	)
	repoPolicy := &redant.Struct[RepoPolicy]{Value: RepoPolicy{
		ProtectedBranches: []string{"main", "release"},
		RequireReview:     true,
		MinApprovals:      2,
	}}
	repoCreateCmd := &redant.Command{
		Use:   "create",
		Short: "Create repository with policy.",
		Long:  "Create repository under project scope with enum and struct options.",
		Options: redant.OptionSet{
			{Flag: "visibility", Description: "Repo visibility.", Value: redant.EnumOf(&repoVisibility, "public", "private", "internal"), Default: "private"},
			{Flag: "tags", Description: "Repo tags.", Value: redant.StringArrayOf(&repoTags)},
			{Flag: "policy", Description: "Repo policy object.", Value: repoPolicy},
			{Flag: "mirror", Description: "Mirror upstream URL.", Value: redant.URLOf(&repoMirrorURL), Default: "https://github.com/pubgo/redant"},
		},
		Args: redant.ArgSet{
			{Name: "repo_name", Required: true, Description: "Repository name.", Value: redant.StringOf(&repoName)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			fmt.Printf("[project repo create] name=%s visibility=%s tags=%v mirror=%s\n", repoName, repoVisibility, repoTags, redant.URLOf(&repoMirrorURL).String())
			fmt.Printf("[project repo create] policy=%s\n", toJSON(repoPolicy.Value))
			return nil
		},
	}

	var (
		mirrorName  string
		mirrorMode  string
		mirrorForce bool
	)
	projectRepoMirrorCmd := &redant.Command{
		Use:   "mirror",
		Short: "Mirror repository.",
		Long:  "Mirror repository with enum mode and bool options.",
		Options: redant.OptionSet{
			{Flag: "mode", Description: "Mirror mode.", Value: redant.EnumOf(&mirrorMode, "fetch", "push", "bidirectional"), Default: "fetch"},
			{Flag: "force", Description: "Force mirror sync.", Value: redant.BoolOf(&mirrorForce)},
		},
		Args: redant.ArgSet{{Name: "repo_name", Required: true, Description: "Repository name.", Value: redant.StringOf(&mirrorName)}},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			fmt.Printf("[project repo mirror] name=%s mode=%s force=%v\n", mirrorName, mirrorMode, mirrorForce)
			return nil
		},
	}

	projectRepoCmd := &redant.Command{
		Use:   "repo",
		Short: "Repository operations.",
		Long:  "Repository operations for integration tests.",
		Children: []*redant.Command{
			repoCreateCmd,
			projectRepoMirrorCmd,
		},
	}

	var (
		envName    string
		envTargets []string
	)
	projectEnvPromoteCmd := &redant.Command{
		Use:   "promote",
		Short: "Promote environment.",
		Long:  "Promote env with enum-array targets.",
		Options: redant.OptionSet{
			{Flag: "targets", Description: "Promotion targets.", Value: redant.EnumArrayOf(&envTargets, "staging", "pre", "prod")},
		},
		Args: redant.ArgSet{{Name: "env", Required: true, Description: "Environment name.", Value: redant.StringOf(&envName)}},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			fmt.Printf("[project env promote] env=%s targets=%v\n", envName, envTargets)
			return nil
		},
	}

	projectEnvCmd := &redant.Command{
		Use:   "env",
		Short: "Environment operations.",
		Children: []*redant.Command{
			projectEnvPromoteCmd,
		},
	}

	projectCmd := &redant.Command{
		Use:   "project",
		Short: "Project operations.",
		Long:  "Project command group with 3-level subcommands for completion and web testing.",
		Children: []*redant.Command{
			projectRepoCmd,
			projectEnvCmd,
		},
	}

	var (
		profileName    string
		profileContent string
	)
	profileCmd := &redant.Command{
		Use:   "profile",
		Short: "Profile parser playground.",
		Long:  "Playground command for args formats (query/form/json/positional).",
		Metadata: map[string]string{
			agentlinemodule.CommandMetaAgentCommand: "true",
		},
		Args: redant.ArgSet{
			{Name: "name", Required: true, Description: "Profile name.", Value: redant.StringOf(&profileName)},
			{Name: "content", Required: false, Description: "Profile content.", Value: redant.StringOf(&profileContent)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			fmt.Printf("[profile] args=%v name=%s content=%s\n", inv.Args, profileName, profileContent)
			return nil
		},
	}

	commitCmd.Children = append(commitCmd.Children, detailedCmd)

	rootCmd.Children = append(rootCmd.Children,
		commitCmd,
		releaseShipCmd,
		projectCmd,
		profileCmd,
		completioncmd.New(),
		readlinecmd.New(),
		richlinecmd.New(),
		mcpcmd.New(),
		webcmd.New(),
		webttycmd.New(),
	)

	rootCmd.Handler = func(ctx context.Context, inv *redant.Invocation) error {
		return agentlineapp.Run(ctx, rootCmd, &agentlineapp.RuntimeOptions{
			Prompt: "agent> ",
			Stdin:  inv.Stdin,
			Stdout: inv.Stdout,
		})
	}

	err := rootCmd.Invoke().WithOS().Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
