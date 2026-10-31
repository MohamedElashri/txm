// Package supervisor owns one-use launch manifests and immutable exit receipts.
package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/metrics"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

const MaxManifest = 128 << 10

type Manifest struct {
	SchemaVersion int             `json:"schema_version"`
	ManagedID     string          `json:"managed_id"`
	InstanceID    string          `json:"instance_id"`
	Backend       string          `json:"backend"`
	Argv          []string        `json:"argv"`
	Cwd           string          `json:"cwd"`
	EnvNames      []string        `json:"env_names"`
	Receipt       string          `json:"receipt"`
	Hooks         *HookContext    `json:"hooks,omitempty"`
	Metrics       *metrics.Config `json:"metrics,omitempty"`
}
type HookContext struct {
	AgentID    string `json:"agent_id"`
	InstanceID string `json:"instance_id"`
	Generation string `json:"generation"`
	StateHome  string `json:"state_home"`
	RuntimeDir string `json:"runtime_dir"`
	TempDir    string `json:"temp_dir"`
}

func (h HookContext) Environment() []string {
	return []string{"TXM_AGENT_ID=" + h.AgentID, "TXM_AGENT_INSTANCE_ID=" + h.InstanceID, "TXM_HOOK_GENERATION=" + h.Generation, "XDG_STATE_HOME=" + h.StateHome, "XDG_RUNTIME_DIR=" + h.RuntimeDir, "TMPDIR=" + h.TempDir}
}

type Receipt struct {
	SchemaVersion int        `json:"schema_version"`
	ManagedID     string     `json:"managed_id"`
	InstanceID    string     `json:"instance_id"`
	Backend       string     `json:"backend"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    time.Time  `json:"finished_at"`
	Outcome       string     `json:"outcome"`
	ExitCode      *int       `json:"exit_code"`
	Signal        *string    `json:"signal"`
}
type Status struct {
	SchemaVersion int       `json:"schema_version"`
	ManagedID     string    `json:"managed_id"`
	InstanceID    string    `json:"instance_id"`
	Running       bool      `json:"running"`
	ObservedAt    time.Time `json:"observed_at"`
}

func ManifestName(id string) string { return "m-" + id + ".json" }
func ReceiptName(id string) string  { return "r-" + id + ".json" }
func State() (*runtimepath.Root, error) {
	r, e := runtimepath.OpenState()
	if e != nil {
		return nil, e
	}
	defer func() { _ = r.Close() }()
	return r.Child("agents")
}
func Runtime() (*runtimepath.Root, error) {
	r, e := runtimepath.Open()
	if e != nil {
		return nil, e
	}
	defer func() { _ = r.Close() }()
	return r.Child("agents")
}

var envName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func (m Manifest) Validate() error {
	bad := func() error { return core.Failure("invalid_usage", "invalid managed launch manifest") }
	if m.Metrics != nil && m.Metrics.Validate() != nil {
		return bad()
	}
	if m.Hooks != nil {
		h := m.Hooks
		if h.AgentID != m.ManagedID || h.InstanceID != m.InstanceID || !runtimepath.ValidID(h.Generation) || !filepath.IsAbs(h.StateHome) || !filepath.IsAbs(h.TempDir) || (h.RuntimeDir != "" && !filepath.IsAbs(h.RuntimeDir)) {
			return bad()
		}
	}
	if m.SchemaVersion != 1 || !runtimepath.ValidID(m.ManagedID) || !runtimepath.ValidID(m.InstanceID) || m.ManagedID == m.InstanceID || (m.Backend != "native" && m.Backend != "tmux") || m.Receipt != ReceiptName(m.InstanceID) {
		return bad()
	}
	if len(m.Argv) == 0 || len(m.Argv) > 256 || !filepath.IsAbs(m.Argv[0]) || !filepath.IsAbs(m.Cwd) || len(m.EnvNames) > 128 {
		return bad()
	}
	if strings.ContainsRune(m.Cwd, 0) || !utf8.ValidString(m.Cwd) {
		return bad()
	}
	for _, v := range m.Argv {
		if strings.ContainsRune(v, 0) || !utf8.ValidString(v) {
			return bad()
		}
	}
	seen := map[string]bool{}
	for _, name := range m.EnvNames {
		if !envName.MatchString(name) || len(name) > 128 || seen[name] {
			return bad()
		}
		seen[name] = true
	}
	data, e := json.Marshal(m)
	if e != nil || len(data) > MaxManifest {
		return bad()
	}
	return nil
}
func WriteManifest(r *runtimepath.Root, m Manifest) error {
	if e := m.Validate(); e != nil {
		return e
	}
	data, _ := json.Marshal(m)
	return r.WritePrivate(ManifestName(m.InstanceID), data, false)
}
func ReadReceipt(r *runtimepath.Root, managed, id, backend string) (*Receipt, error) {
	if !runtimepath.ValidID(managed) || !runtimepath.ValidID(id) {
		return nil, core.Failure("invalid_config", "invalid receipt identity")
	}
	data, e := r.ReadPrivate(ReceiptName(id), 4096)
	if e != nil {
		return nil, e
	}
	var v Receipt
	if nativeprotocol.Decode(data, &v) != nil || v.SchemaVersion != 1 || v.ManagedID != managed || v.InstanceID != id || v.Backend != backend || v.FinishedAt.IsZero() {
		return nil, core.Failure("invalid_config", "corrupt or mismatched managed receipt")
	}
	valid := false
	switch v.Outcome {
	case "exited":
		valid = v.ExitCode != nil && *v.ExitCode >= 0 && *v.ExitCode <= 255 && v.Signal == nil && v.StartedAt != nil
	case "signaled":
		valid = v.ExitCode == nil && v.Signal != nil && *v.Signal != "" && v.StartedAt != nil
	case "start_failed":
		valid = v.ExitCode == nil && v.Signal == nil && v.StartedAt == nil
	}
	if !valid || (v.StartedAt != nil && (v.StartedAt.IsZero() || v.FinishedAt.Before(*v.StartedAt))) {
		return nil, core.Failure("invalid_config", "invalid managed receipt outcome")
	}
	return &v, nil
}
func ChildEnvironment(names []string) []string {
	result := []string{}
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			result = append(result, name+"="+value)
		}
	}
	return result
}
func Now() time.Time { return time.Now().UTC() }
