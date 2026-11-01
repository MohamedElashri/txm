// Package agent owns the opt-in coordinator and its private registry.
package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/metrics"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

type Record struct {
	ID         string                 `json:"id"`
	ManagedID  *string                `json:"managed_id"`
	InstanceID *string                `json:"instance_id"`
	Locator    core.SessionLocator    `json:"locator"`
	Binding    backend.ManagedBinding `json:"binding"`
	Supervised bool                   `json:"supervised"`
	CreatedAt  time.Time              `json:"created_at"`
	Hook       *HookState             `json:"hook,omitempty"`
}

var tmuxID = regexp.MustCompile(`^\$[0-9]+$`)

type View struct {
	ID                  string              `json:"id"`
	ManagedID           *string             `json:"managed_id"`
	InstanceID          *string             `json:"instance_id"`
	Locator             core.SessionLocator `json:"locator"`
	Supervised          bool                `json:"supervised"`
	Lifecycle           string              `json:"lifecycle"`
	Attention           string              `json:"attention"`
	AttentionSource     string              `json:"attention_source"`
	AttentionObservedAt *time.Time          `json:"attention_observed_at"`
	Source              string              `json:"source"`
	ObservedAt          time.Time           `json:"observed_at"`
	Receipt             *supervisor.Receipt `json:"receipt"`
	Hooks               *HookInfo           `json:"hooks,omitempty"`
	TurnCompleteAt      *time.Time          `json:"turn_complete_at"`
}
type Registry struct {
	SchemaVersion int      `json:"schema_version"`
	HostID        string   `json:"host_id"`
	Agents        []Record `json:"agents"`
}
type Request struct {
	DryRun          bool            `json:"dry_run,omitempty"`
	Confirm         string          `json:"confirm,omitempty"`
	SchemaVersion   int             `json:"schema_version"`
	Epoch           string          `json:"epoch"`
	HostID          string          `json:"host_id"`
	StateRoot       string          `json:"state_root"`
	Op              string          `json:"op"`
	ID              string          `json:"id"`
	Backend         string          `json:"backend"`
	Name            string          `json:"name"`
	Argv            []string        `json:"argv"`
	Cwd             string          `json:"cwd"`
	EnvNames        []string        `json:"env_names"`
	Provider        string          `json:"provider"`
	ProviderVersion string          `json:"provider_version"`
	Signal          *Signal         `json:"signal"`
	ProviderEvent   *ProviderEvent  `json:"provider_event,omitempty"`
	Metrics         *metrics.Config `json:"metrics,omitempty"`
	LastEpoch       string          `json:"last_epoch"`
	LastSequence    uint64          `json:"last_sequence"`
}
type Reply struct {
	SchemaVersion int                  `json:"schema_version"`
	Epoch         string               `json:"epoch"`
	Agents        []View               `json:"agents"`
	Error         *core.OperationError `json:"error"`
	Event         *Event               `json:"event,omitempty"`
}
type Discovery struct {
	SchemaVersion int    `json:"schema_version"`
	Epoch         string `json:"epoch"`
}

func (q Request) validate() error {
	bad := func() error { return core.Failure("invalid_usage", "invalid agent request fields") }
	if q.Op == "start-metrics" {
		if q.Metrics == nil || q.Metrics.Validate() != nil {
			return bad()
		}
	} else if q.Metrics != nil {
		return bad()
	}
	if q.Backend != "" && q.Backend != "native" && q.Backend != "tmux" && q.Backend != "screen" && q.Backend != "zellij" {
		return bad()
	}
	if q.Op != "prune" && (q.DryRun || q.Confirm != "") {
		return bad()
	}
	switch q.Op {
	case "subscribe":
		if q.ID != "" && !runtimepath.ValidID(q.ID) {
			return bad()
		}
		if q.Name != "" || q.Backend != "" || len(q.Argv) > 0 || q.Cwd != "" || len(q.EnvNames) > 0 || q.Provider != "" || q.Signal != nil {
			return bad()
		}
	case "hooks-enable", "hooks-info":
		if !runtimepath.ValidID(q.ID) || q.Name != "" || q.Backend != "" || len(q.Argv) > 0 || q.Cwd != "" || len(q.EnvNames) > 0 || q.Signal != nil {
			return bad()
		}
	case "signal", "provider-event":
		if !runtimepath.ValidID(q.ID) || q.Signal == nil || q.Name != "" || q.Backend != "" || len(q.Argv) > 0 || q.Cwd != "" || len(q.EnvNames) > 0 || q.Provider != "" {
			return bad()
		}
		if q.Op == "provider-event" && (q.ProviderEvent == nil || q.ProviderEvent.Kind == "signal") {
			return bad()
		}
	case "start", "start-metrics":
		if (q.ID != "" && !runtimepath.ValidID(q.ID)) || q.Name == "" || q.Backend == "" || (q.Provider == "" && q.ProviderVersion != "") {
			return bad()
		}
	case "prune":
		if !runtimepath.ValidID(q.ID) || (!q.DryRun && q.Confirm != q.ID) || (q.Confirm != "" && q.Confirm != q.ID) || q.Name != "" || len(q.Argv) > 0 || q.Cwd != "" || len(q.EnvNames) > 0 {
			return bad()
		}
	case "terminate":
		if !runtimepath.ValidID(q.ID) || q.Name != "" || q.Backend != "" || len(q.Argv) > 0 || q.Cwd != "" || len(q.EnvNames) > 0 {
			return bad()
		}
	case "register":
		if q.ID != "" || q.Name == "" || q.Backend == "" || len(q.Argv) > 0 || q.Cwd != "" || len(q.EnvNames) > 0 {
			return bad()
		}
	case "list", "status", "stop", "ping", "refresh":
		if q.Name != "" || len(q.Argv) > 0 || q.Cwd != "" || len(q.EnvNames) > 0 {
			return bad()
		}
		if q.Op != "status" && q.ID != "" {
			return bad()
		}
		if q.Op != "list" && q.Op != "status" && q.Backend != "" {
			return bad()
		}
	default:
		return bad()
	}
	if q.Op != "signal" && q.Op != "provider-event" && q.Signal != nil {
		return bad()
	}
	if q.Op != "provider-event" && q.ProviderEvent != nil {
		return bad()
	}
	if q.Op != "subscribe" && (q.LastEpoch != "" || q.LastSequence != 0) {
		return bad()
	}
	if q.Op != "start" && q.Op != "start-metrics" && q.Op != "hooks-enable" && (q.Provider != "" || q.ProviderVersion != "") {
		return bad()
	}
	return nil
}

func (r Record) validate(host string) error {
	bad := func() error { return core.Failure("invalid_config", "corrupt agent registry") }
	if r.Hook != nil && !r.Hook.validate(r) {
		return bad()
	}
	if !runtimepath.ValidID(r.ID) || r.Locator.HostID != host || r.Locator.Name == "" || len(r.Locator.Name) > 512 || strings.ContainsRune(r.Locator.Name, 0) || r.CreatedAt.IsZero() || (r.Locator.Backend != "native" && r.Locator.Backend != "tmux") {
		return bad()
	}
	if r.Supervised {
		if r.ManagedID == nil || *r.ManagedID != r.ID || r.InstanceID == nil || !runtimepath.ValidID(*r.InstanceID) || *r.InstanceID == r.ID {
			return bad()
		}
	} else if r.ManagedID != nil || r.InstanceID != nil {
		return bad()
	}
	if r.Supervised && r.Binding == (backend.ManagedBinding{}) {
		return nil
	} // Persisted pre-launch intent; recovery is unknown.
	if r.Locator.Backend == "native" && (!runtimepath.ValidID(r.Binding.NativeInstance) || r.Binding.BackendID != r.Binding.NativeInstance || r.Binding.Owner != "") {
		return bad()
	}
	if r.Locator.Backend == "tmux" && (r.Binding.NativeInstance != "" || !runtimepath.ValidID(r.Binding.Owner)) {
		return bad()
	}
	if r.Locator.Backend == "tmux" && !tmuxID.MatchString(r.Binding.BackendID) && (!r.Supervised || r.Binding.BackendID != "") {
		return bad()
	}
	return nil
}
func LoadRegistry(state *runtimepath.Root, host string) (Registry, error) {
	result := Registry{SchemaVersion: 1, HostID: host, Agents: []Record{}}
	data, e := state.ReadPrivate("registry.json", 1<<20)
	if os.IsNotExist(e) {
		return result, nil
	}
	if e != nil {
		return result, e
	}
	result = Registry{} // Missing required fields must not inherit default values.
	if nativeprotocol.Decode(data, &result) != nil || result.SchemaVersion != 1 || result.HostID != host || result.Agents == nil || len(result.Agents) > 256 {
		return result, core.Failure("invalid_config", "corrupt or incompatible agent registry")
	}
	seen := map[string]bool{}
	for _, record := range result.Agents {
		if record.validate(host) != nil || seen[record.ID] {
			return result, core.Failure("invalid_config", "corrupt or duplicate agent registry identity")
		}
		seen[record.ID] = true
	}
	return result, nil
}
func saveRegistry(state *runtimepath.Root, r Registry) error {
	data, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return state.WritePrivate("registry.json", data, true)
}

// CompileStart resolves executable/cwd in the requesting CLI; no shell or
// backend/default command ever interprets the user's argv.
func CompileStart(name, backendName, cwd string, argv, env []string) (supervisor.Manifest, error) {
	m := supervisor.Manifest{SchemaVersion: 1, ManagedID: "00000000000000000000000000000001", InstanceID: "00000000000000000000000000000002", Backend: backendName, Cwd: cwd, Argv: argv, EnvNames: env, Receipt: supervisor.ReceiptName("00000000000000000000000000000002")}
	if runtimepath.ValidateName(name) != nil {
		return m, core.Failure("invalid_usage", "invalid agent session name")
	}
	if cwd == "" {
		var e error
		cwd, e = os.Getwd()
		if e != nil {
			return m, e
		}
	}
	abs, e := filepath.Abs(cwd)
	if e != nil {
		return m, e
	}
	abs, e = filepath.EvalSymlinks(abs)
	if e != nil {
		return m, core.Failure("invalid_usage", "agent cwd does not exist")
	}
	info, e := os.Stat(abs)
	if e != nil || !info.IsDir() {
		return m, core.Failure("invalid_usage", "agent cwd must be a directory")
	}
	m.Cwd = abs
	if e = m.Validate(); e != nil {
		return m, e
	}
	return m, nil
}
func sortedViews(v []View) { sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID }) }
func Context(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 15*time.Second)
}
