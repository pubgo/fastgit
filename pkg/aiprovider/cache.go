package aiprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pubgo/fastgit/configs"
)

type cacheEntry struct {
	Text     string    `json:"text"`
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	SavedAt  time.Time `json:"saved_at"`
}

type cachedProvider struct {
	inner Provider
	dir   string
}

// WithCache wraps a provider with a local disk cache keyed by prompt hash.
func WithCache(inner Provider) Provider {
	if inner == nil {
		return inner
	}
	dir := filepath.Join(filepath.Dir(configs.GetConfigPath()), "ai-cache")
	return &cachedProvider{inner: inner, dir: dir}
}

func (p *cachedProvider) Name() string {
	if p.inner == nil {
		return "cache"
	}
	return p.inner.Name() + "+cache"
}

func (p *cachedProvider) Available() bool {
	return p.inner != nil && p.inner.Available()
}

func (p *cachedProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	key := cacheKey(req)
	if entry, ok := p.load(key); ok {
		return CompleteResponse{
			Text:     entry.Text,
			Provider: entry.Provider,
			Model:    entry.Model,
		}, nil
	}

	resp, err := p.inner.Complete(ctx, req)
	if err != nil || resp.Fallback || strings.TrimSpace(resp.Text) == "" {
		return resp, err
	}
	_ = p.save(key, cacheEntry{
		Text:     resp.Text,
		Provider: resp.Provider,
		Model:    resp.Model,
		SavedAt:  time.Now(),
	})
	return resp, err
}

func cacheKey(req CompleteRequest) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(req.System) + "\n---\n" + strings.TrimSpace(req.User)))
	return hex.EncodeToString(sum[:])
}

func (p *cachedProvider) load(key string) (cacheEntry, bool) {
	path := filepath.Join(p.dir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheEntry{}, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return cacheEntry{}, false
	}
	return entry, strings.TrimSpace(entry.Text) != ""
}

func (p *cachedProvider) save(key string, entry cacheEntry) error {
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.dir, key+".json"), raw, 0o644)
}
