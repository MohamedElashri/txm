package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

type SetupPolicy struct {
	Schema       int      `json:"schema_version"`
	TaskID       string   `json:"task_id"`
	Name         string   `json:"name"`
	Repository   string   `json:"repository"`
	Worktree     string   `json:"worktree"`
	Backend      string   `json:"backend"`
	BaseCommit   string   `json:"base_commit"`
	MainArgv     []string `json:"main_argv"`
	MainEnvNames []string `json:"main_env_names"`
	Argv         []string `json:"setup_argv"`
	TimeoutMS    int      `json:"timeout_ms"`
}
type setupGrant struct {
	Policy              SetupPolicy
	Digest              string
	ConfigHash          string
	SetupHash, MainHash string
	GitHash, GitPath    string
}
type SetupState struct {
	Schema     int                 `json:"schema_version"`
	TaskID     string              `json:"task_id"`
	ManagedID  string              `json:"managed_id"`
	InstanceID string              `json:"instance_id"`
	Digest     string              `json:"digest"`
	State      string              `json:"state"`
	Receipt    *supervisor.Receipt `json:"receipt"`
	MainLaunch bool                `json:"main_launch_intent"`
}

func hashBytes(b []byte) string                   { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func setupExecutable(path string) (string, error) { return extension.ExecutableHash(path) }
func prepareSetup(ctx context.Context, g git, o Options, repo, base string) (*setupGrant, error) {
	if o.SetupPolicy == "" {
		if o.AcceptSetup != "" {
			return nil, core.Failure("invalid_usage", "setup acceptance requires a policy")
		}
		return nil, nil
	}
	if o.AllowRepoCommands || o.Branch != "" {
		return nil, core.Failure("denied", "reviewed setup requires hooks/filters disabled and the ID-derived task branch")
	}
	if !filepath.IsAbs(o.SetupPolicy) || extension.NoSymlinks(o.SetupPolicy) != nil {
		return nil, core.Failure("denied", "setup policy must be an absolute owned private file")
	}
	if _, e := extension.Identify(o.SetupPolicy); e != nil {
		return nil, e
	}
	f, e := os.Open(o.SetupPolicy)
	if e != nil {
		return nil, e
	}
	defer func() { _ = f.Close() }()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 64<<10 {
		return nil, core.Failure("denied", "setup policy requires mode 0600 and at most 64 KiB")
	}
	b, e := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if e != nil {
		return nil, e
	}
	var p SetupPolicy
	if nativeprotocol.Decode(b, &p) != nil || p.Schema != 1 || !runtimepath.ValidID(p.TaskID) || p.Name != o.Name || p.Repository != repo || p.Backend != o.Backend || p.BaseCommit != base || !slices.Equal(p.MainArgv, o.Argv) || !slices.Equal(p.MainEnvNames, o.Env) || p.TimeoutMS < 1 || p.TimeoutMS > 60000 || !filepath.IsAbs(p.Worktree) || filepath.Base(p.Worktree) != p.TaskID || len(p.Argv) == 0 {
		return nil, core.Failure("denied", "setup policy does not match exact task request")
	}
	if _, e = agent.CompileStart(p.Name, p.Backend, repo, p.Argv, nil); e != nil {
		return nil, e
	}
	setupHash, e := setupExecutable(p.Argv[0])
	if e != nil {
		return nil, e
	}
	mainHash, e := setupExecutable(p.MainArgv[0])
	if e != nil {
		return nil, e
	}
	gitHash, e := extension.ExecutableHash(g.exe)
	if e != nil {
		return nil, e
	}
	ri, e := extension.Identify(repo)
	if e != nil {
		return nil, e
	}
	config, _, e := g.configuration(ctx, repo)
	if e != nil {
		return nil, e
	}
	canonical, _ := json.Marshal(struct {
		Policy                   SetupPolicy
		Path                     string
		Repo                     extension.Identity
		Config, Setup, Main, Git string
	}{p, o.SetupPolicy, ri, hashBytes(config), setupHash, mainHash, gitHash})
	grant := &setupGrant{Policy: p, Digest: hashBytes(canonical), ConfigHash: hashBytes(config), SetupHash: setupHash, MainHash: mainHash, GitHash: gitHash, GitPath: g.exe}
	if !o.DryRun && o.AcceptSetup != grant.Digest {
		return nil, core.Failure("denied", "setup requires the exact digest from a reviewed dry run")
	}
	return grant, nil
}
func (s *store) runSetup(ctx context.Context, g git, r Record, grant *setupGrant) error {
	if e := grant.verifyExecutables(); e != nil {
		return e
	}
	if r.Worktree != grant.Policy.Worktree {
		return core.Failure("conflict", "reviewed setup worktree path changed")
	}
	config, _, e := g.configuration(ctx, r.Worktree)
	if e != nil {
		return e
	}
	if hashBytes(config) != grant.ConfigHash {
		return core.Failure("conflict", "new worktree configuration differs from reviewed setup")
	}
	if e = grant.verifyExecutables(); e != nil {
		return e
	}
	id, e := runtimepath.NewID()
	if e != nil {
		return e
	}
	state := SetupState{Schema: 1, TaskID: r.ID, ManagedID: id, Digest: grant.Digest, State: "uncertain"}
	save := func() error { b, _ := json.Marshal(state); return s.root.WritePrivate("setup-"+r.ID+".json", b, true) }
	if e = save(); e != nil {
		return e
	}
	bounded, cancel := context.WithTimeout(ctx, time.Duration(grant.Policy.TimeoutMS)*time.Millisecond)
	defer cancel()
	reply, e := agent.Call(bounded, agent.Request{Op: "start", ID: id, Name: "txm-setup-" + r.ID, Backend: r.Backend, Cwd: r.Worktree, Argv: grant.Policy.Argv}, true)
	if len(reply.Agents) == 1 && reply.Agents[0].ID == id && reply.Agents[0].InstanceID != nil {
		state.InstanceID = *reply.Agents[0].InstanceID
		if err := save(); err != nil {
			return err
		}
	}
	if e != nil {
		return core.Failure("conflict", "setup outcome uncertain; main job not launched, inspect retained setup intent")
	}
	v, e := agent.WaitExisting(bounded, core.ReadScope{Backends: []string{r.Backend}, ManagedIDs: []string{id}}, id, "exited", time.Duration(grant.Policy.TimeoutMS)*time.Millisecond)
	if e != nil {
		return core.Failure("conflict", "setup interrupted or timed out; inspect retained setup job before cleanup")
	}
	state.Receipt = v.Receipt
	state.State = "failed"
	if v.Receipt != nil && v.Receipt.ExitCode != nil && *v.Receipt.ExitCode == 0 {
		state.State = "succeeded"
	}
	if e = save(); e != nil {
		return e
	}
	if state.State != "succeeded" {
		return core.Failure("operation_failed", "setup failed; main job not launched, worktree retained")
	}
	if e = grant.verifyExecutables(); e != nil {
		return e
	}
	config, _, e = g.configuration(ctx, r.Worktree)
	if e != nil {
		return e
	}
	if hashBytes(config) != grant.ConfigHash {
		return core.Failure("conflict", "setup changed reviewed Git configuration; main job not launched")
	}
	return nil
}
func (g *setupGrant) verifyExecutables() error {
	gitHash, e := extension.ExecutableHash(g.GitPath)
	if e != nil || gitHash != g.GitHash {
		return core.Failure("conflict", "reviewed Git executable changed")
	}
	s, e := setupExecutable(g.Policy.Argv[0])
	if e != nil || s != g.SetupHash {
		return core.Failure("conflict", "reviewed setup executable changed")
	}
	m, e := setupExecutable(g.Policy.MainArgv[0])
	if e != nil || m != g.MainHash {
		return core.Failure("conflict", "reviewed main executable changed")
	}
	return nil
}
func readSetup(root *runtimepath.Root, id string) (*SetupState, error) {
	b, e := root.ReadPrivate("setup-"+id+".json", 8192)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var state SetupState
	if nativeprotocol.Decode(b, &state) != nil || state.Schema != 1 || state.TaskID != id || !runtimepath.ValidID(state.ManagedID) || len(state.Digest) != 64 || (state.InstanceID != "" && !runtimepath.ValidID(state.InstanceID)) || (state.State != "uncertain" && state.State != "failed" && state.State != "succeeded") {
		return nil, core.Failure("invalid_config", "invalid task setup state")
	}
	if state.State == "succeeded" && (state.Receipt == nil || state.Receipt.ManagedID != state.ManagedID || state.Receipt.InstanceID != state.InstanceID || state.Receipt.ExitCode == nil || *state.Receipt.ExitCode != 0) {
		return nil, core.Failure("conflict", "setup success has no pinned receipt")
	}
	return &state, nil
}

func SetupStatus(ctx context.Context, id string) (*SetupState, error) {
	if _, e := taskRecordExisting(ctx, id); e != nil {
		return nil, e
	}
	state, e := runtimepath.OpenExistingState()
	if e != nil {
		return nil, e
	}
	defer func() { _ = state.Close() }()
	r, e := state.ChildExisting("tasks")
	if e != nil {
		return nil, e
	}
	defer func() { _ = r.Close() }()
	return readSetup(r, id)
}
func guardSetupExit(ctx context.Context, root *runtimepath.Root, r Record) error {
	s, e := readSetup(root, r.ID)
	if e != nil {
		return e
	}
	if s == nil {
		return nil
	}
	if s.InstanceID == "" {
		return core.Failure("conflict", "setup has uncertain launch ownership; inspect setup intent before cleanup")
	}
	pin := r
	pin.ID = s.ManagedID
	pin.InstanceID = s.InstanceID
	if _, e = ReadExistingReceipt(ctx, pin); e != nil {
		return core.Failure("conflict", "setup has no pinned exit receipt; cleanup refused")
	}
	return nil
}

func markSetupMain(root *runtimepath.Root, id string) error {
	s, e := readSetup(root, id)
	if e != nil {
		return e
	}
	if s == nil || s.State != "succeeded" || s.MainLaunch {
		return core.Failure("conflict", "setup cannot authorize main launch")
	}
	s.MainLaunch = true
	b, _ := json.Marshal(s)
	return root.WritePrivate("setup-"+id+".json", b, true)
}
