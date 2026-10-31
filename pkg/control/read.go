// Package control composes scoped business operations for CLI, MCP and UI.
package control

import (
	"context"
	"os"
	"slices"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/supervisor"
	"github.com/MohamedElashri/txm/pkg/task"
)

type Reader struct {
	scope        core.ReadScope
	repositories map[string]extension.Identity
}

func NewReader(scope core.ReadScope) (*Reader, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	scope.Backends = slices.Clone(scope.Backends)
	scope.ManagedIDs = slices.Clone(scope.ManagedIDs)
	scope.Repositories = slices.Clone(scope.Repositories)
	r := &Reader{scope: scope, repositories: map[string]extension.Identity{}}
	for _, repo := range scope.Repositories {
		identity, err := extension.Identify(repo)
		if err != nil {
			return nil, core.Failure("invalid_config", "read repository unavailable or unsafe")
		}
		info, err := os.Stat(repo)
		if err != nil || !info.IsDir() {
			return nil, core.Failure("invalid_config", "read repository must be an owned directory")
		}
		r.repositories[repo] = identity
	}
	return r, nil
}

func (r *Reader) scoped(ctx context.Context, id string) (core.ReadScope, error) {
	if err := ctx.Err(); err != nil {
		return core.ReadScope{}, err
	}
	scope := r.scope
	if id != "" {
		if scope.Empty() {
			return scope, core.Failure("denied", "resource is outside read scope")
		}
		if len(scope.ManagedIDs) > 0 && !slices.Contains(scope.ManagedIDs, id) {
			return scope, core.Failure("denied", "resource is outside read scope")
		}
		scope.ManagedIDs = []string{id}
		if err := scope.Validate(); err != nil {
			return scope, core.Failure("denied", "invalid scoped resource identity")
		}
	}
	for repo, expected := range r.repositories {
		current, err := extension.Identify(repo)
		if err != nil || current != expected {
			return scope, core.Failure("conflict", "approved repository identity changed")
		}
	}
	return scope, nil
}

func (r *Reader) agentScope(ctx context.Context, id string) (core.ReadScope, map[string]string, error) {
	scope, err := r.scoped(ctx, id)
	if err != nil {
		return scope, nil, err
	}
	pins := map[string]string{}
	if len(scope.Repositories) > 0 {
		records, err := task.ReadExistingRecords(ctx, scope)
		if err != nil {
			return scope, nil, err
		}
		scope.ManagedIDs = []string{}
		for _, record := range records {
			// A launch intent without a pinned instance cannot authorize probing
			// whatever process later happens to claim that managed ID.
			if record.InstanceID == "" {
				continue
			}
			scope.ManagedIDs = append(scope.ManagedIDs, record.ID)
			pins[record.ID] = record.InstanceID
		}
		scope.Repositories = nil
	}
	return scope, pins, nil
}

func (r *Reader) Agents(ctx context.Context) ([]agent.PublicView, error) {
	scope, pins, err := r.agentScope(ctx, "")
	if err != nil {
		return nil, err
	}
	return agent.ReadPinnedExisting(ctx, scope, pins)
}

func (r *Reader) Agent(ctx context.Context, id string) (agent.PublicView, error) {
	scope, pins, err := r.agentScope(ctx, id)
	if err != nil {
		return agent.PublicView{}, err
	}
	views, err := agent.ReadPinnedExisting(ctx, scope, pins)
	if err != nil {
		return agent.PublicView{}, err
	}
	if len(views) != 1 {
		return agent.PublicView{}, core.Failure("not_found", "scoped agent is unavailable")
	}
	return views[0], nil
}

type SessionView struct {
	AgentID           string              `json:"agent_id"`
	Locator           core.SessionLocator `json:"locator"`
	ManagedInstanceID *string             `json:"managed_instance_id"`
	ProcessLifecycle  string              `json:"process_lifecycle"`
	SessionStatus     string              `json:"session_status"`
	Source            string              `json:"source"`
}

func sessionView(v agent.PublicView) SessionView {
	return SessionView{AgentID: v.ID, Locator: v.Locator, ManagedInstanceID: v.InstanceID, ProcessLifecycle: v.Lifecycle, SessionStatus: "unverified", Source: "managed registration; backend session not probed"}
}

// Sessions reports only authorized managed registrations. This deliberately
// makes no attach/capture or unmanaged-session discovery promise.
func (r *Reader) Sessions(ctx context.Context) ([]SessionView, error) {
	views, err := r.Agents(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SessionView, 0, len(views))
	for _, v := range views {
		out = append(out, sessionView(v))
	}
	return out, nil
}

func (r *Reader) Session(ctx context.Context, id string) (SessionView, error) {
	v, err := r.Agent(ctx, id)
	if err != nil {
		return SessionView{}, err
	}
	return sessionView(v), nil
}

type TaskView struct {
	ManagedID  string              `json:"managed_id"`
	InstanceID *string             `json:"instance_id"`
	Worktree   *string             `json:"worktree"`
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	HostID     string              `json:"host_id"`
	Backend    string              `json:"backend"`
	Repository *string             `json:"repository"`
	BaseCommit string              `json:"base_commit"`
	Cleaned    bool                `json:"cleaned"`
	Lifecycle  string              `json:"lifecycle"`
	Source     string              `json:"source"`
	Receipt    *supervisor.Receipt `json:"receipt"`
}

func (r *Reader) taskViews(ctx context.Context, id string) ([]TaskView, error) {
	scope, err := r.scoped(ctx, id)
	if err != nil {
		return nil, err
	}
	records, err := task.ReadExistingRecords(ctx, scope)
	if err != nil {
		return nil, err
	}
	out := make([]TaskView, 0, len(records))
	for _, record := range records {
		v := TaskView{ID: record.ID, Name: record.Name, HostID: record.HostID, Backend: record.Backend, BaseCommit: record.Base, Cleaned: record.Cleaned, Lifecycle: "unknown", Source: "existing task metadata; no exit proof"}
		v.ManagedID = record.ID
		if record.InstanceID != "" {
			instance := record.InstanceID
			v.InstanceID = &instance
		}
		if slices.Contains(scope.Repositories, record.Repo) {
			repo := record.Repo
			v.Repository = &repo
			worktree := record.Worktree
			v.Worktree = &worktree
		}
		if record.InstanceID != "" {
			pinned := core.ReadScope{HostID: record.HostID, Backends: []string{record.Backend}, ManagedIDs: []string{record.ID}}
			views, err := agent.ReadPinnedExisting(ctx, pinned, map[string]string{record.ID: record.InstanceID})
			if err != nil {
				return nil, err
			}
			if len(views) == 1 {
				v.Lifecycle, v.Source, v.Receipt = views[0].Lifecycle, views[0].Source, views[0].Receipt
			} else {
				receipt, err := task.ReadExistingReceipt(ctx, record)
				if err == nil {
					v.Lifecycle, v.Source, v.Receipt = "exited", "pinned supervisor receipt", receipt
				} else if !os.IsNotExist(err) {
					return nil, core.Failure("conflict", "task exit proof unavailable or unsafe")
				}
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *Reader) Tasks(ctx context.Context) ([]TaskView, error) { return r.taskViews(ctx, "") }
func (r *Reader) Task(ctx context.Context, id string) (TaskView, error) {
	views, err := r.taskViews(ctx, id)
	if err != nil {
		return TaskView{}, err
	}
	if len(views) != 1 {
		return TaskView{}, core.Failure("not_found", "scoped task is unavailable")
	}
	return views[0], nil
}

func (r *Reader) Wait(ctx context.Context, id, until string, timeout time.Duration) (agent.PublicView, error) {
	return agent.WaitForObservation(ctx, func(ctx context.Context) (agent.PublicView, error) { return r.Agent(ctx, id) }, until, timeout)
}
