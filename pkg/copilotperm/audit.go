package copilotperm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adrg/xdg"
)

// AuditEntry records one permission decision.
type AuditEntry struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"session_id,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	ToolName  string    `json:"tool_name,omitempty"`
	Decision  string    `json:"decision"`
	Mode      Mode      `json:"mode"`
	Summary   string    `json:"summary,omitempty"`
}

// Auditor persists permission audit events.
type Auditor interface {
	Log(entry AuditEntry)
}

// FileAuditor appends JSON lines to a local audit log.
type FileAuditor struct {
	path string
	mu   sync.Mutex
}

// NewFileAuditor creates an auditor writing to the default XDG path.
func NewFileAuditor() (*FileAuditor, error) {
	return &FileAuditor{path: filepath.Join(xdg.ConfigHome, "fastgit", "audit.log")}, nil
}

// Log appends one audit entry as a JSON line.
func (a *FileAuditor) Log(entry AuditEntry) {
	if a == nil {
		return
	}
	if entry.Time.IsZero() {
		entry.Time = time.Now().UTC()
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return
	}

	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintln(f, string(data))
}
