package task

import (
	"context"
	"encoding/json"
	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"os"
	"path/filepath"
	"regexp"
)

type Record struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	HostID            string             `json:"host_id"`
	StateRoot         string             `json:"state_root"`
	Backend           string             `json:"backend"`
	Repo              string             `json:"repo"`
	Common            string             `json:"common_dir"`
	RepoID            extension.Identity `json:"repo_identity"`
	CommonID          extension.Identity `json:"common_identity"`
	Worktree          string             `json:"worktree"`
	WorktreeID        extension.Identity `json:"worktree_identity"`
	GitFileID         extension.Identity `json:"git_file_identity"`
	GitFile           string             `json:"git_file"`
	Admin             string             `json:"admin"`
	AdminID           extension.Identity `json:"admin_identity"`
	Branch            string             `json:"branch"`
	Base              string             `json:"base_commit"`
	AllowRepoCommands bool               `json:"allow_repo_commands"`
	Created           bool               `json:"created"`
	LaunchRequested   bool               `json:"launch_requested"`
	InstanceID        string             `json:"instance_id,omitempty"`
	Cleaned           bool               `json:"cleaned"`
}
type View struct {
	Task   Record               `json:"task"`
	Agent  *agent.View          `json:"agent"`
	Orphan bool                 `json:"orphan"`
	Error  *core.OperationError `json:"error"`
}
type index struct {
	Schema    int      `json:"schema_version"`
	HostID    string   `json:"host_id"`
	StateRoot string   `json:"state_root"`
	Tasks     []Record `json:"tasks"`
}
type store struct {
	root *runtimepath.Root
	lock *os.File
	data index
}

var names = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func open(ctx context.Context) (*store, error) {
	host, e := runtimepath.HostID(ctx)
	if e != nil {
		return nil, e
	}
	r, l, e := extension.Store(ctx, "tasks")
	if e != nil {
		return nil, e
	}
	s := &store{root: r, lock: l, data: index{1, host, filepath.Dir(r.Path), []Record{}}}
	b, e := r.ReadPrivate("index.json", 1<<20)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e == nil {
		e = nativeprotocol.Decode(b, &s.data)
	}
	if e == nil {
		e = s.validate(host)
	}
	if e != nil {
		s.close()
		return nil, core.Failure("invalid_config", "unsafe or corrupt task metadata")
	}
	return s, nil
}
func (s *store) close() { _ = s.lock.Close(); _ = s.root.Close() }
func (s *store) validate(host string) error {
	d := s.data
	if d.Schema != 1 || d.HostID != host || d.StateRoot != filepath.Dir(s.root.Path) || d.Tasks == nil || len(d.Tasks) > 128 {
		return core.Failure("invalid_config", "task store identity mismatch")
	}
	seen := map[string]bool{}
	for _, r := range d.Tasks {
		if !runtimepath.ValidID(r.ID) || seen[r.ID] || !names.MatchString(r.Name) || r.HostID != host || r.StateRoot != d.StateRoot || (r.Backend != "native" && r.Backend != "tmux") || r.Worktree != filepath.Join(d.StateRoot, "task-worktrees", r.ID) || !commitPattern.MatchString(r.Base) || !filepath.IsAbs(r.Repo) || !filepath.IsAbs(r.Common) || r.Branch == "" || r.RepoID.Inode == 0 || r.CommonID.Inode == 0 {
			return core.Failure("invalid_config", "invalid task record")
		}
		if r.Created && (r.WorktreeID.Inode == 0 || r.GitFileID.Inode == 0 || r.AdminID.Inode == 0 || !filepath.IsAbs(r.Admin) || r.GitFile == "") {
			return core.Failure("invalid_config", "incomplete worktree identity")
		}
		if r.LaunchRequested && !r.Created {
			return core.Failure("invalid_config", "invalid task launch intent")
		}
		if r.InstanceID != "" && (!r.LaunchRequested || !runtimepath.ValidID(r.InstanceID) || r.InstanceID == r.ID) {
			return core.Failure("invalid_config", "invalid task supervisor instance")
		}
		seen[r.ID] = true
	}
	return nil
}
func (s *store) save() error {
	if e := s.validate(s.data.HostID); e != nil {
		return e
	}
	b, e := json.Marshal(s.data)
	if e != nil {
		return e
	}
	return s.root.WritePrivate("index.json", b, true)
}
func (s *store) find(id string) (*Record, error) {
	if !runtimepath.ValidID(id) {
		return nil, core.Failure("invalid_target", "task operation requires a full opaque ID")
	}
	for i := range s.data.Tasks {
		if s.data.Tasks[i].ID == id {
			return &s.data.Tasks[i], nil
		}
	}
	return nil, core.Failure("not_found", "task ID not found")
}
func observe(ctx context.Context, r Record) (View, error) {
	v := View{Task: r}
	if !r.LaunchRequested {
		v.Orphan = true
		return v, nil
	}
	reply, e := agent.Call(ctx, agent.Request{Op: "status", ID: r.ID}, true)
	if e != nil {
		if core.Normalize(e).Code == "not_found" {
			return recoverReceipt(r)
		}
		v.Error = core.Normalize(e)
		return v, e
	}
	if len(reply.Agents) != 1 {
		return v, core.Failure("protocol_error", "task agent response missing")
	}
	a := reply.Agents[0]
	if !matchesAgent(r, a) {
		return v, core.Failure("conflict", "task managed identity mismatch")
	}
	v.Agent = &a
	return v, nil
}
func List(ctx context.Context) ([]View, error) {
	s, e := open(ctx)
	if e != nil {
		return nil, e
	}
	defer s.close()
	out := []View{}
	for _, r := range s.data.Tasks {
		v, err := observe(ctx, r)
		if err != nil {
			v.Error = core.Normalize(err)
		}
		out = append(out, v)
	}
	return out, nil
}
func Inspect(ctx context.Context, id string) (View, error) {
	s, e := open(ctx)
	if e != nil {
		return View{}, e
	}
	defer s.close()
	r, e := s.find(id)
	if e != nil {
		return View{}, e
	}
	return observe(ctx, *r)
}
func Stop(ctx context.Context, id string) (View, error) {
	s, e := open(ctx)
	if e != nil {
		return View{}, e
	}
	defer s.close()
	r, e := s.find(id)
	if e != nil {
		return View{}, e
	}
	v, e := observe(ctx, *r)
	if e != nil {
		return v, e
	}
	if v.Orphan {
		return v, core.Failure("not_found", "task has no managed launch to stop")
	}
	if v.Agent != nil && v.Agent.Receipt != nil {
		return v, nil
	}
	reply, e := agent.Call(ctx, agent.Request{Op: "terminate", ID: r.ID}, true)
	if len(reply.Agents) == 1 {
		v.Agent = &reply.Agents[0]
	}
	return v, e
}
