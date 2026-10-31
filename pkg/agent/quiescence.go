package agent

import (
	"context"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

// AssertManagedQuiescence is deliberately conservative: v1 registrations have
// no durable cwd provenance, so they cannot prove an unrelated writer safe.
// Every registered worker must have exit proof. This cannot prove that escaped
// descendants or external editors stopped; the operator must assert that too.
func AssertManagedQuiescence(ctx context.Context) error {
	host, e := runtimepath.ExistingHostID(ctx)
	if e != nil {
		return e
	}
	state, e := runtimepath.OpenExistingState()
	if e != nil {
		return e
	}
	defer func() { _ = state.Close() }()
	r, e := state.ChildExisting("agents")
	if e != nil {
		return e
	}
	defer func() { _ = r.Close() }()
	registry, e := LoadRegistry(r, host)
	if e != nil {
		return e
	}
	for _, record := range registry.Agents {
		if e = ctx.Err(); e != nil {
			return e
		}
		if !record.Supervised || record.InstanceID == nil {
			return core.Failure("conflict", "unverified registered writer prevents quiescent merge")
		}
		if _, e = supervisor.ReadReceipt(r, record.ID, *record.InstanceID, record.Locator.Backend); e != nil {
			return core.Failure("conflict", "all registered workers require exit receipts before merge; writer cwd provenance is unavailable")
		}
	}
	return nil
}
