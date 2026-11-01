package task

import (
	"context"
	"os"
	"path/filepath"
	"slices"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

// ReadExistingRecords is a pure metadata snapshot. It does not take the
// creating task-store writer lock or contact/restart a coordinator. Repositories
// are positively bound to their stored filesystem identities before use.
func ReadExistingRecords(ctx context.Context, scope core.ReadScope) ([]Record, error) {
	out := []Record{}
	if err := scope.Validate(); err != nil {
		return out, err
	}
	if scope.Empty() {
		return out, nil
	}
	if !supervisor.Supported() {
		return out, core.Failure("unsupported", "task reads require Unix and CGO")
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	host, err := runtimepath.ExistingHostID(ctx)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, core.Failure("invalid_config", "existing local identity unavailable or unsafe")
	}
	if scope.HostID != "" && scope.HostID != host {
		return out, core.Failure("denied", "host is outside read scope")
	}
	state, err := runtimepath.OpenExistingState()
	if err != nil {
		return out, core.Failure("invalid_config", "existing task state unavailable or unsafe")
	}
	defer func() { _ = state.Close() }()
	root, err := state.ChildExisting("tasks")
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, core.Failure("invalid_config", "existing task store unavailable or unsafe")
	}
	defer func() { _ = root.Close() }()
	b, err := root.ReadPrivate("index.json", 1<<20)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, core.Failure("invalid_config", "existing task index unavailable or unsafe")
	}
	s := store{root: root}
	if nativeprotocol.Decode(b, &s.data) != nil || s.validate(host) != nil {
		return out, core.Failure("invalid_config", "unsafe or incompatible task metadata")
	}
	for _, r := range s.data.Tasks {
		if !scope.Allows(host, r.Backend, r.ID, r.Repo) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return out, err
		}
		for _, target := range []struct {
			path     string
			identity extension.Identity
		}{{r.Repo, r.RepoID}, {r.Common, r.CommonID}} {
			identity, err := extension.Identify(target.path)
			if err != nil || identity != target.identity {
				return out, core.Failure("conflict", "scoped task repository identity changed")
			}
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Record) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out, nil
}

// ReadExistingReceipt recovers only the immutable receipt pinned by an owned
// task record, never by name or a guessed replacement instance.
func ReadExistingReceipt(ctx context.Context, r Record) (*supervisor.Receipt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !runtimepath.ValidID(r.ID) || !runtimepath.ValidID(r.InstanceID) {
		return nil, core.Failure("conflict", "task has no pinned supervisor exit proof")
	}
	state, err := runtimepath.OpenExistingState()
	if err != nil {
		return nil, err
	}
	defer func() { _ = state.Close() }()
	if state.Path != r.StateRoot {
		return nil, core.Failure("conflict", "task state namespace differs")
	}
	root, err := state.ChildExisting("agents")
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	if root.Path != filepath.Join(r.StateRoot, "agents") {
		return nil, core.Failure("conflict", "task agent namespace differs")
	}
	return supervisor.ReadReceipt(root, r.ID, r.InstanceID, r.Backend)
}
