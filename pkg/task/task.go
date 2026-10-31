package task

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Name, Repo, Base, Branch, Backend string
	// ReservedID is for durable service reservations, never a CLI name or retry.
	ReservedID                string
	SetupPolicy, AcceptSetup  string
	Argv, Env                 []string
	DryRun, AllowRepoCommands bool
	TrustedTools              bool
}
type Plan struct {
	DryRun        bool     `json:"dry_run"`
	Name          string   `json:"name"`
	Repo          string   `json:"repo"`
	Base          string   `json:"base_commit"`
	Branch        string   `json:"branch"`
	Backend       string   `json:"backend"`
	ArgumentCount int      `json:"argument_count"`
	EnvNames      []string `json:"env_names"`
	SetupDigest   string   `json:"setup_digest,omitempty"`
	ReservedID    string   `json:"reserved_id,omitempty"`
	SetupWorktree string   `json:"setup_worktree,omitempty"`
}

func Start(ctx context.Context, o Options) (any, error) {
	if o.ReservedID != "" && !runtimepath.ValidID(o.ReservedID) {
		return nil, core.Failure("invalid_usage", "invalid reserved task identity")
	}
	if !names().MatchString(o.Name) || o.Repo == "" || (o.Backend != "native" && o.Backend != "tmux") {
		return nil, core.Failure("invalid_usage", "task requires a bounded ASCII name, --repo and native/tmux backend")
	}
	if !supervisor.Supported() {
		return nil, core.Failure("unsupported", "tasks require Unix and CGO")
	}
	g, e := newGit(o.AllowRepoCommands)
	if e != nil {
		return nil, e
	}
	if o.TrustedTools || o.SetupPolicy != "" {
		if _, e = extension.ExecutableHash(g.exe); e != nil {
			return nil, e
		}
	}
	repo, common, e := g.repo(ctx, o.Repo)
	if e != nil {
		return nil, e
	}
	if e = g.filters(ctx, repo); e != nil {
		return nil, e
	}
	if o.Base == "" {
		o.Base = "HEAD"
	}
	base, e := g.text(ctx, repo, "rev-parse", "--verify", "--end-of-options", o.Base+"^{commit}")
	if e != nil || !commitPattern().MatchString(base) {
		return nil, core.Failure("invalid_target", "task base must resolve to a committed revision")
	}
	branch := o.Branch
	if branch == "" {
		branch = "txm/" + o.Name + "-<new-id>"
	} else {
		if e = checkBranch(ctx, g, repo, branch); e != nil {
			return nil, e
		}
	}
	m, e := agent.CompileStart(o.Name, o.Backend, repo, o.Argv, o.Env)
	if e != nil {
		return nil, e
	}
	setup, e := prepareSetup(ctx, g, o, repo, base)
	if e != nil {
		return nil, e
	}
	if setup != nil {
		o.ReservedID = setup.Policy.TaskID
	}
	if o.DryRun {
		p := Plan{DryRun: true, Name: o.Name, Repo: repo, Base: base, Branch: branch, Backend: o.Backend, ArgumentCount: len(m.Argv), EnvNames: m.EnvNames}
		if setup != nil {
			p.SetupDigest = setup.Digest
			p.ReservedID = setup.Policy.TaskID
			p.SetupWorktree = setup.Policy.Worktree
		}
		return p, nil
	}
	b, e := backend.NewBackend(o.Backend, ctx)
	if e != nil {
		return nil, e
	}
	if e = backend.PreflightManaged(ctx, b); e != nil {
		return nil, e
	}
	if _, e = b.Lookup(ctx, o.Name); e == nil {
		return nil, core.Failure("conflict", "task session name already exists")
	} else if core.Normalize(e).Code != "not_found" {
		return nil, e
	}
	s, e := open(ctx)
	if e != nil {
		return nil, e
	}
	defer s.close()
	if len(s.data.Tasks) >= 128 {
		return nil, core.Failure("conflict", "task metadata limit reached")
	}
	id, e := runtimepath.NewID()
	if e != nil {
		return nil, e
	}
	if o.ReservedID != "" {
		id = o.ReservedID
		for _, prior := range s.data.Tasks {
			if prior.ID == id {
				return nil, core.Failure("conflict", "reserved task identity already used; inspect existing outcome")
			}
		}
	}
	if o.Branch == "" {
		branch = "txm/" + o.Name + "-" + id
	}
	if e = checkBranch(ctx, g, repo, branch); e != nil {
		return nil, e
	}
	ri, e := extension.Identify(repo)
	if e != nil {
		return nil, e
	}
	ci, e := extension.Identify(common)
	if e != nil {
		return nil, e
	}
	state, e := runtimepath.OpenState()
	if e != nil {
		return nil, e
	}
	wt, e := state.Child("task-worktrees")
	_ = state.Close()
	if e != nil {
		return nil, e
	}
	defer func() { _ = wt.Close() }()
	dest := filepath.Join(wt.Path, id)
	if setup != nil && dest != setup.Policy.Worktree {
		return nil, core.Failure("conflict", "reviewed setup worktree does not match task state namespace")
	}
	if _, e = os.Lstat(dest); !os.IsNotExist(e) {
		return nil, core.Failure("conflict", "task worktree destination already exists")
	}
	r := Record{ID: id, Name: o.Name, HostID: s.data.HostID, StateRoot: s.data.StateRoot, Backend: o.Backend, Repo: repo, Common: common, RepoID: ri, CommonID: ci, Worktree: dest, Branch: branch, Base: base, AllowRepoCommands: o.AllowRepoCommands}
	s.data.Tasks = append(s.data.Tasks, r)
	record := &s.data.Tasks[len(s.data.Tasks)-1]
	if e = s.save(); e != nil {
		return nil, e
	}
	add := []string{"worktree", "add"}
	if !g.allow {
		add = append(add, "--no-checkout")
	}
	add = append(add, "-b", branch, "--", dest, base)
	if _, _, e = g.run(ctx, repo, add...); e != nil {
		return View{Task: *record, Orphan: true}, e
	}
	// Record ownership before any launch intent; never infer it from a name later.
	if e = capture(ctx, g, record); e != nil {
		return View{Task: *record, Orphan: true}, e
	}
	record.Created = true
	if e = s.save(); e != nil {
		return View{Task: *record, Orphan: true}, e
	}
	// Conditional Git config may apply only in the new worktree. Inspect it
	// before populating files, so filters cannot execute during checkout.
	if e = verify(ctx, g, *record); e != nil {
		return View{Task: *record, Orphan: true}, e
	}
	if !g.allow {
		entries, err := os.ReadDir(dest)
		if err != nil {
			return View{Task: *record, Orphan: true}, err
		}
		if len(entries) != 1 || entries[0].Name() != ".git" {
			return View{Task: *record, Orphan: true}, core.Failure("conflict", "new worktree contents changed before checkout")
		}
		if _, _, e = g.run(ctx, dest, "read-tree", "--reset", "-u", "HEAD"); e != nil {
			return View{Task: *record, Orphan: true}, e
		}
	}
	if setup != nil {
		// Reserve the workflow launch in v1 metadata before setup. Older
		// binaries then refuse cleanup without a task receipt instead of
		// mistaking an active setup worker for an unused orphan worktree.
		record.LaunchRequested = true
		if e = s.save(); e != nil {
			return View{Task: *record, Orphan: true}, e
		}
		if e = s.runSetup(ctx, g, *record, setup); e != nil {
			return View{Task: *record, Orphan: true}, e
		}
		if e = verify(ctx, g, *record); e != nil {
			return View{Task: *record, Orphan: true}, e
		}
		if e = markSetupMain(s.root, record.ID); e != nil {
			return View{Task: *record, Orphan: true}, e
		}
	}
	record.LaunchRequested = true
	if e = s.save(); e != nil {
		return View{Task: *record, Orphan: true}, e
	}
	reply, e := agent.Call(ctx, agent.Request{Op: "start", ID: id, Name: o.Name, Backend: o.Backend, Argv: m.Argv, Cwd: dest, EnvNames: m.EnvNames}, true)
	v := View{Task: *record}
	if len(reply.Agents) == 1 {
		a := reply.Agents[0]
		if !matchesAgent(*record, a) {
			return v, core.Failure("protocol_error", "task start managed identity mismatch")
		}
		record.InstanceID = *a.InstanceID
		v.Task = *record
		v.Agent = &a
		if saveErr := s.save(); saveErr != nil {
			return v, saveErr
		}
	} else if e == nil {
		return v, core.Failure("protocol_error", "task start managed reply missing")
	}
	return v, e
}
func checkBranch(ctx context.Context, g git, repo, branch string) error {
	if len(branch) > 240 || strings.HasPrefix(branch, "-") {
		return core.Failure("invalid_target", "invalid task branch")
	}
	if _, _, e := g.run(ctx, repo, "check-ref-format", "refs/heads/"+branch); e != nil {
		return core.Failure("invalid_target", "invalid task branch")
	}
	_, code, e := g.run(ctx, repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if code == 0 {
		return core.Failure("conflict", "task branch already exists")
	}
	if code != 1 {
		return e
	}
	return nil
}
func capture(ctx context.Context, g git, r *Record) error {
	var e error
	r.WorktreeID, e = extension.Identify(r.Worktree)
	if e != nil {
		return e
	}
	p := filepath.Join(r.Worktree, ".git")
	r.GitFileID, e = extension.Identify(p)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	if len(b) > 4096 || !strings.HasPrefix(string(b), "gitdir: ") {
		return core.Failure("conflict", "unexpected task Git file")
	}
	r.GitFile = string(b)
	r.Admin, e = g.text(ctx, r.Worktree, "rev-parse", "--absolute-git-dir")
	if e != nil {
		return e
	}
	r.AdminID, e = extension.Identify(r.Admin)
	return e
}
func verify(ctx context.Context, g git, r Record) error {
	for _, p := range []struct {
		path string
		id   extension.Identity
	}{{r.Repo, r.RepoID}, {r.Common, r.CommonID}, {r.Worktree, r.WorktreeID}, {filepath.Join(r.Worktree, ".git"), r.GitFileID}, {r.Admin, r.AdminID}} {
		id, e := extension.Identify(p.path)
		if e != nil {
			return e
		}
		if id != p.id {
			return core.Failure("conflict", "task filesystem identity changed")
		}
	}
	root, common, e := g.repo(ctx, r.Repo)
	if e != nil {
		return e
	}
	if root != r.Repo || common != r.Common {
		return core.Failure("conflict", "task repository routing changed")
	}
	workroot, wc, e := g.repo(ctx, r.Worktree)
	if e != nil {
		return e
	}
	admin, e := g.text(ctx, r.Worktree, "rev-parse", "--absolute-git-dir")
	if e != nil {
		return e
	}
	data, e := os.ReadFile(filepath.Join(r.Worktree, ".git"))
	if e != nil {
		return e
	}
	if string(data) != r.GitFile || workroot != r.Worktree || wc != r.Common || admin != r.Admin {
		return core.Failure("conflict", "task worktree routing changed")
	}
	gitdir, e := os.ReadFile(filepath.Join(r.Admin, "gitdir"))
	if e != nil || strings.TrimSuffix(string(gitdir), "\n") != filepath.Join(r.Worktree, ".git") {
		return core.Failure("conflict", "task Git administrative link changed")
	}
	branch, e := g.text(ctx, r.Worktree, "symbolic-ref", "--quiet", "HEAD")
	if e != nil || branch != "refs/heads/"+r.Branch {
		return core.Failure("conflict", "task worktree branch changed")
	}
	if e = g.filters(ctx, r.Repo); e != nil {
		return e
	}
	return g.filters(ctx, r.Worktree)
}
func Cleanup(ctx context.Context, id string, dry, force bool, confirm string) (View, error) {
	if (force && confirm != id) || (!force && confirm != "") {
		return View{}, core.Failure("invalid_usage", "force cleanup requires --force --confirm FULL_TASK_ID")
	}
	s, e := open(ctx)
	if e != nil {
		return View{}, e
	}
	defer s.close()
	r, e := s.find(id)
	if e != nil {
		return View{}, e
	}
	if e = guardSetupExit(ctx, s.root, *r); e != nil {
		return View{Task: *r}, e
	}
	v := View{Task: *r}
	setupState, setupErr := readSetup(s.root, r.ID)
	if setupErr != nil {
		return v, setupErr
	}
	if setupState == nil || setupState.MainLaunch {
		v, e = observe(ctx, *r)
		if e != nil {
			return v, e
		}
	} else {
		v.Orphan = true
	}
	if v.Agent != nil && (v.Agent.Lifecycle != "exited" || v.Agent.Receipt == nil) {
		return v, core.Failure("conflict", "task cleanup requires a durable exit receipt")
	}
	if r.Cleaned {
		return v, nil
	}
	if !r.Created {
		return v, core.Failure("conflict", "incomplete worktree ownership; manual inspection required")
	}
	g, e := newGit(r.AllowRepoCommands)
	if e != nil {
		return v, e
	}
	if e = verify(ctx, g, *r); e != nil {
		return v, e
	}
	if !force {
		dirty, _, e := g.run(ctx, r.Worktree, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching")
		if e != nil {
			return v, e
		}
		if len(dirty) > 0 {
			return v, core.Failure("conflict", "task worktree has modified, untracked or ignored files")
		}
	}
	if dry {
		return v, nil
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, "--", r.Worktree)
	if _, _, e = g.run(ctx, r.Repo, args...); e != nil {
		return v, e
	}
	r.Cleaned = true
	v.Task = *r
	return v, s.save()
}

// Prune removes metadata only after cleanup; receipts, branches and agent state
// remain independent. A replacement at the old worktree path blocks pruning.
func Prune(ctx context.Context, id string, dry bool, confirm string) (View, error) {
	if (!dry && confirm != id) || (confirm != "" && confirm != id) {
		return View{}, core.Failure("invalid_usage", "prune requires --confirm FULL_TASK_ID or --dry-run")
	}
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
	if !r.Cleaned {
		return v, core.Failure("conflict", "task must be cleaned before metadata pruning")
	}
	if v.Agent != nil && (v.Agent.Lifecycle != "exited" || v.Agent.Receipt == nil) {
		return v, core.Failure("conflict", "task prune requires exit proof")
	}
	if e = extension.NoSymlinks(r.Worktree); e != nil {
		return v, e
	}
	if _, e = os.Lstat(r.Worktree); !os.IsNotExist(e) {
		return v, core.Failure("conflict", "task worktree path is present or inaccessible")
	}
	if dry {
		return v, nil
	}
	next := make([]Record, 0, len(s.data.Tasks)-1)
	for _, record := range s.data.Tasks {
		if record.ID != id {
			next = append(next, record)
		}
	}
	s.data.Tasks = next
	return v, s.save()
}
