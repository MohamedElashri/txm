// Package core defines backend-independent observations and public contracts.
package core

import (
	"context"
	"time"
)

const MaxBytes = 1 << 20
const RequestTimeout = 5 * time.Second

type ManagedID string
type SessionLocator struct {
	HostID  string `json:"host_id"`
	Backend string `json:"backend"`
	Name    string `json:"name"`
}
type Capability struct {
	Level       string    `json:"level"`
	Reason      *string   `json:"reason"`
	Constraints []string  `json:"constraints"`
	Source      string    `json:"source"`
	ObservedAt  time.Time `json:"observed_at"`
}
type Capabilities map[string]Capability
type BackendCapabilities struct {
	Backend      string       `json:"backend"`
	Available    bool         `json:"available"`
	Version      *string      `json:"version"`
	Capabilities Capabilities `json:"capabilities"`
}
type Session struct {
	Locator         SessionLocator `json:"locator"`
	BackendID       *string        `json:"backend_id"`
	ManagedID       *ManagedID     `json:"managed_id"`
	InstanceID      *string        `json:"instance_id"`
	ProcessState    string         `json:"process_state"`
	ClientState     string         `json:"client_state"`
	AttachedClients *int           `json:"attached_clients"`
	ObservedAt      time.Time      `json:"observed_at"`
	Source          string         `json:"source"`
	Capabilities    Capabilities   `json:"capabilities"`
}
type Window struct {
	ID, Name string
	Index    int
}
type Pane struct {
	ID, WindowID  string
	Index, Height int
}
type CaptureOptions struct {
	Lines, Bytes int
	Raw, History bool
}

func (o CaptureOptions) Validate() error {
	if o.Lines < 1 || o.Lines > 10000 || o.Bytes < 1 || o.Bytes > MaxBytes {
		return Failure("invalid_usage", "output limits must be lines 1..10000 and bytes 1..1048576")
	}
	return nil
}

type Snapshot struct {
	Locator    SessionLocator `json:"locator"`
	Type       string         `json:"type"`
	Format     string         `json:"format"`
	Content    string         `json:"content"`
	Encoding   *string        `json:"encoding,omitempty"`
	Lines      int            `json:"lines"`
	Bytes      int            `json:"bytes"`
	Truncated  bool           `json:"truncated"`
	ObservedAt time.Time      `json:"observed_at"`
}
type Reader interface {
	Sessions(context.Context) ([]Session, error)
	Lookup(context.Context, string) (Session, error)
	Capture(context.Context, string, CaptureOptions) (Snapshot, error)
	BackendCapabilities(context.Context) (BackendCapabilities, error)
	Windows(context.Context, string) ([]Window, error)
	Panes(context.Context, string, string) ([]Pane, error)
}
type Envelope struct {
	SchemaVersion int               `json:"schema_version"`
	Data          any               `json:"data"`
	Errors        []*OperationError `json:"errors"`
	Partial       bool              `json:"partial"`
}

func String(s string) *string { return &s }
