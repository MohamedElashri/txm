package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/task"
)

// CreatePolicy is an operator-owned startup grant. Tool arguments can select
// a template but cannot supply commands, names, repositories or environment.
// The initial implementation supplies an empty child environment deliberately.
type CreatePolicy struct {
	SchemaVersion int              `json:"schema_version"`
	Namespace     string           `json:"namespace"`
	Templates     []CreateTemplate `json:"templates"`
}
type CreateTemplate struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Backend    string   `json:"backend"`
	Directory  string   `json:"directory"`
	BaseCommit string   `json:"base_commit,omitempty"`
	Argv       []string `json:"argv"`
}
type Creator struct {
	policy CreatePolicy
	digest string
	pins   map[string]templatePin
}
type templatePin struct {
	Directory  extension.Identity
	Executable string
}
type CreateInput struct {
	OperationID string `json:"operation_id"`
	TemplateID  string `json:"template_id"`
}
type Creation struct {
	OperationID string `json:"operation_id"`
	ResourceID  string `json:"resource_id"`
	Kind        string `json:"kind"`
	State       string `json:"state"`
	// No raw argv, environment, configuration or provider payload is retained.
	RequestHash string `json:"request_hash"`
}
type createJournal struct {
	Schema  int        `json:"schema_version"`
	Policy  string     `json:"policy_hash"`
	HostID  string     `json:"host_id"`
	Entries []Creation `json:"entries"`
}

var templateName = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`) })

func digestJSON(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func executableDigest(path string) (string, error) { return extension.ExecutableHash(path) }
func LoadCreator(path string) (*Creator, error) {
	if !filepath.IsAbs(path) || extension.NoSymlinks(path) != nil {
		return nil, core.Failure("denied", "create policy requires an owned absolute private file")
	}
	if _, err := extension.Identify(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 64<<10 {
		return nil, core.Failure("denied", "create policy requires mode 0600 and at most 64 KiB")
	}
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil {
		return nil, err
	}
	var p CreatePolicy
	if nativeprotocol.Decode(b, &p) != nil {
		return nil, core.Failure("invalid_config", "invalid create policy")
	}
	return NewCreator(p)
}
func NewCreator(p CreatePolicy) (*Creator, error) {
	if p.SchemaVersion != 1 || !runtimepath.ValidID(p.Namespace) || len(p.Templates) == 0 || len(p.Templates) > 32 {
		return nil, core.Failure("invalid_config", "invalid create policy bounds")
	}
	// Clone nested slices: callers cannot change the startup grant afterwards.
	b, _ := json.Marshal(p)
	_ = json.Unmarshal(b, &p)
	c := &Creator{policy: p, pins: map[string]templatePin{}}
	for _, t := range p.Templates {
		if !templateName().MatchString(t.ID) || !templateName().MatchString(t.Name) || (t.Kind != "agent" && t.Kind != "task") || (t.Backend != "native" && t.Backend != "tmux") || len(t.Argv) == 0 {
			return nil, core.Failure("invalid_config", "invalid create template")
		}
		if _, exists := c.pins[t.ID]; exists {
			return nil, core.Failure("invalid_config", "duplicate create template")
		}
		if !filepath.IsAbs(t.Directory) || filepath.Clean(t.Directory) != t.Directory {
			return nil, core.Failure("invalid_config", "template directory must be canonical and absolute")
		}
		dir, err := extension.Identify(t.Directory)
		if err != nil {
			return nil, err
		}
		if _, err = agent.CompileStart(t.Name, t.Backend, t.Directory, t.Argv, nil); err != nil {
			return nil, err
		}
		exe, err := executableDigest(t.Argv[0])
		if err != nil {
			return nil, err
		}
		if t.Kind == "task" && !regexp.MustCompile(`^([a-f0-9]{40}|[a-f0-9]{64})$`).MatchString(t.BaseCommit) {
			return nil, core.Failure("invalid_config", "task template requires a frozen full base commit")
		}
		if t.Kind == "agent" && t.BaseCommit != "" {
			return nil, core.Failure("invalid_config", "agent template cannot set a Git base")
		}
		c.pins[t.ID] = templatePin{dir, exe}
	}
	c.digest = digestJSON(struct {
		Policy CreatePolicy
		Pins   map[string]templatePin
	}{p, c.pins})
	return c, nil
}
func (c *Creator) Allows(kind string) bool {
	for _, t := range c.policy.Templates {
		if t.Kind == kind {
			return true
		}
	}
	return false
}

// Create reserves before touching a backend or Git. Every subsequent call for
// an ID returns its stored result, including uncertain intent, without retry.
// Entries are permanent tombstones; there is no eviction or namespace reuse.
func (c *Creator) Create(ctx context.Context, kind string, in CreateInput) (Creation, error) {
	var empty Creation
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if !runtimepath.ValidID(in.OperationID) {
		return empty, core.Failure("invalid_usage", "operation requires a full opaque ID")
	}
	var chosen *CreateTemplate
	for i := range c.policy.Templates {
		t := &c.policy.Templates[i]
		if t.ID == in.TemplateID && t.Kind == kind {
			chosen = t
			break
		}
	}
	if chosen == nil {
		return empty, core.Failure("denied", "create template is outside startup grant")
	}
	t := *chosen
	hash := digestJSON(struct{ Policy, Kind, Template string }{c.digest, kind, t.ID})
	host, err := runtimepath.HostID(ctx)
	if err != nil {
		return empty, err
	}
	r, lock, err := extension.Store(ctx, "mcp-"+c.policy.Namespace)
	if err != nil {
		return empty, err
	}
	defer func() { _ = lock.Close(); _ = r.Close() }()
	j := createJournal{1, c.digest, host, []Creation{}}
	b, err := r.ReadPrivate("operations.json", 1<<20)
	if err == nil {
		if nativeprotocol.Decode(b, &j) != nil || j.Schema != 1 || j.Policy != c.digest || j.HostID != host || j.Entries == nil || len(j.Entries) > 1024 {
			return empty, core.Failure("conflict", "create namespace identity or policy changed")
		}
	} else if !os.IsNotExist(err) {
		return empty, err
	}
	seen := map[string]bool{}
	for _, prior := range j.Entries {
		if !runtimepath.ValidID(prior.OperationID) || !runtimepath.ValidID(prior.ResourceID) || seen[prior.OperationID] || (prior.State != "uncertain" && prior.State != "accepted") || len(prior.RequestHash) != 64 || (prior.Kind != "agent" && prior.Kind != "task") {
			return empty, core.Failure("invalid_config", "invalid create journal")
		}
		seen[prior.OperationID] = true
		if prior.OperationID == in.OperationID {
			if prior.RequestHash != hash {
				return empty, core.Failure("conflict", "operation ID is bound to another request")
			}
			return prior, nil
		}
	}
	if len(j.Entries) >= 1024 {
		return empty, core.Failure("conflict", "create namespace capacity reached; retained operation IDs cannot be reused")
	}
	current, err := extension.Identify(t.Directory)
	if err != nil || current != c.pins[t.ID].Directory {
		return empty, core.Failure("conflict", "template directory identity changed")
	}
	exe, err := executableDigest(t.Argv[0])
	if err != nil || exe != c.pins[t.ID].Executable {
		return empty, core.Failure("conflict", "template executable changed")
	}
	id, err := runtimepath.NewID()
	if err != nil {
		return empty, err
	}
	out := Creation{in.OperationID, id, kind, "uncertain", hash}
	j.Entries = append(j.Entries, out)
	save := func() error {
		b, e := json.Marshal(j)
		if e != nil {
			return e
		}
		if len(b) > 1<<20 {
			return core.Failure("conflict", "create journal byte quota reached")
		}
		return r.WritePrivate("operations.json", b, true)
	}
	if err = save(); err != nil {
		return empty, err
	}
	// Once reserved, cancellation or a lost coordinator response cannot release
	// this ID. Leave uncertain intent for explicit inspection, never relaunch.
	if kind == "agent" {
		_, err = agent.Call(ctx, agent.Request{Op: "start", ID: id, Name: t.Name, Backend: t.Backend, Cwd: t.Directory, Argv: t.Argv}, true)
	} else {
		_, err = task.Start(ctx, task.Options{TrustedTools: true, ReservedID: id, Name: t.Name, Backend: t.Backend, Repo: t.Directory, Base: t.BaseCommit, Argv: t.Argv})
	}
	if err != nil {
		return out, nil
	}
	out.State = "accepted"
	j.Entries[len(j.Entries)-1] = out
	if save() != nil {
		out.State = "uncertain"
	}
	return out, nil
}
