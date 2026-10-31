package task

import (
	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
	"path/filepath"
)

func matchesAgent(r Record, a agent.View) bool {
	return a.ID == r.ID && a.ManagedID != nil && *a.ManagedID == r.ID &&
		a.Supervised && a.Locator.HostID == r.HostID && a.Locator.Backend == r.Backend &&
		a.InstanceID != nil && runtimepath.ValidID(*a.InstanceID) &&
		(r.InstanceID == "" || r.InstanceID == *a.InstanceID)
}

// Registry absence cannot distinguish never launched from deleted metadata.
// Recover only from an independently pinned supervisor instance and its receipt;
// never search manifests/receipts by name or infer completion from missing state.
func recoverReceipt(r Record) (View, error) {
	v := View{Task: r}
	unknown := func() (View, error) {
		e := core.Failure("conflict", "task registration missing; no pinned exit proof; manual recovery required")
		v.Error = e
		return v, e
	}
	if r.InstanceID == "" {
		return unknown()
	}
	state, e := supervisor.State()
	if e != nil {
		return v, e
	}
	defer func() { _ = state.Close() }()
	if state.Path != filepath.Join(r.StateRoot, "agents") {
		return unknown()
	}
	receipt, e := supervisor.ReadReceipt(state, r.ID, r.InstanceID, r.Backend)
	if e != nil {
		return unknown()
	}
	v.Agent = &agent.View{ID: r.ID, ManagedID: core.String(r.ID), InstanceID: core.String(r.InstanceID),
		Locator:    core.SessionLocator{HostID: r.HostID, Backend: r.Backend, Name: r.Name},
		Supervised: true, Lifecycle: "exited", Attention: "unknown", AttentionSource: "no verified hooks",
		Source: "pinned supervisor receipt (registration missing)", ObservedAt: receipt.FinishedAt, Receipt: receipt}
	return v, nil
}
