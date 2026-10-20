package aiprovider

import (
	"context"
	"strings"

	"github.com/pubgo/fastgit/pkg/acpclient"
	"github.com/pubgo/fastgit/pkg/acpconfig"
)

// ACPProvider generates completions by driving an external ACP agent process.
type ACPProvider struct {
	cwd string
}

// NewACP creates an ACP-backed provider. cwd may be empty (defaults to the
// process working directory when the agent session is created).
func NewACP(cwd string) *ACPProvider {
	return &ACPProvider{cwd: cwd}
}

func (p *ACPProvider) Name() string { return "acp" }

// Available reports whether an acp agent can be resolved for use.
func (p *ACPProvider) Available() bool {
	cfg, err := acpconfig.Load()
	if err != nil {
		return false
	}
	_, err = cfg.Resolve("")
	return err == nil
}

// Complete runs one ACP prompt turn via the resolved agent. req.Model, when
// set, selects the agent by name.
func (p *ACPProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	cfg, err := acpconfig.Load()
	if err != nil {
		return CompleteResponse{}, err
	}
	agent, err := cfg.Resolve(req.Model)
	if err != nil {
		return CompleteResponse{}, err
	}

	result, err := acpclient.Run(ctx, agent, acpclient.Options{
		Cwd:    p.cwd,
		System: req.System,
		Prompt: req.User,
	})
	if err != nil {
		return CompleteResponse{}, err
	}
	return CompleteResponse{
		Text:     strings.TrimSpace(result.Text),
		Provider: p.Name(),
		Model:    agent.Name,
	}, nil
}
