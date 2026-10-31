package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"slices"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

// ReadExisting accepts exact managed IDs. A higher-level repository-scoped
// service must first resolve positive task provenance into those exact IDs.
// No coordinator is contacted; registry/receipts are atomic private snapshots.
func ReadExisting(ctx context.Context, scope core.ReadScope) ([]PublicView, error) {
	return ReadPinnedExisting(ctx, scope, nil)
}

// ReadPinnedExisting additionally binds task-derived permissions to a known
// supervisor instance before reading receipts or probing any process.
func ReadPinnedExisting(ctx context.Context, scope core.ReadScope, pins map[string]string) ([]PublicView, error) {
	out := []PublicView{}
	if err := scope.Validate(); err != nil {
		return out, err
	}
	if len(scope.Repositories) > 0 {
		return out, core.Failure("denied", "agent reads require resolved task repository provenance")
	}
	if scope.Empty() {
		return out, nil
	}
	for id, instance := range pins {
		if !slices.Contains(scope.ManagedIDs, id) || !runtimepath.ValidID(instance) {
			return out, core.Failure("invalid_config", "invalid scoped instance pin")
		}
	}
	if !supervisor.Supported() {
		return out, core.Failure("unsupported", "managed observations require Unix and CGO")
	}
	ctx, cancel := context.WithTimeout(ctx, core.RequestTimeout)
	defer cancel()
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
	root, err := runtimepath.OpenExistingState()
	if err != nil {
		return out, core.Failure("invalid_config", "existing state unavailable or unsafe")
	}
	defer func() { _ = root.Close() }()
	state, err := root.ChildExisting("agents")
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, core.Failure("invalid_config", "existing agent state unavailable or unsafe")
	}
	defer func() { _ = state.Close() }()
	registry, err := LoadRegistry(state, host)
	if err != nil {
		return out, err
	}
	for _, r := range registry.Agents {
		if !scope.Allows(host, r.Locator.Backend, r.ID, "") {
			continue
		}
		if instance, ok := pins[r.ID]; ok && (r.InstanceID == nil || *r.InstanceID != instance) {
			return out, core.Failure("conflict", "scoped managed instance changed")
		}
		if err = ctx.Err(); err != nil {
			return out, err
		}
		v := View{ID: r.ID, ManagedID: r.ManagedID, InstanceID: r.InstanceID, Locator: r.Locator, Supervised: r.Supervised, Lifecycle: "unknown", Attention: "unknown", AttentionSource: "no verified hooks", Source: "existing registry; process not verified", ObservedAt: time.Now().UTC()}
		if r.Supervised {
			receipt, readErr := supervisor.ReadReceipt(state, r.ID, *r.InstanceID, r.Locator.Backend)
			if readErr == nil {
				v.Receipt, v.Lifecycle, v.Source, v.ObservedAt = receipt, "exited", "supervisor receipt", receipt.FinishedAt
			} else if !os.IsNotExist(readErr) {
				return out, core.Failure("invalid_config", "scoped exit receipt unavailable or unsafe")
			} else {
				status, probeErr := supervisor.ProbeExisting(ctx, r.ID, *r.InstanceID)
				if ctx.Err() != nil {
					return out, ctx.Err()
				}
				if probeErr == nil && status.Running {
					v.Lifecycle, v.Source, v.ObservedAt = "running", "supervisor kernel observation", status.ObservedAt
				}
				var network net.Error
				if probeErr != nil && !os.IsNotExist(probeErr) && !errors.As(probeErr, &network) && !errors.Is(probeErr, io.EOF) && !errors.Is(probeErr, io.ErrUnexpectedEOF) && !errors.Is(probeErr, context.DeadlineExceeded) {
					return out, core.Failure("invalid_config", "scoped supervisor namespace or identity is unsafe")
				}
			}
		}
		projected, hookErr := projectExternalHook(state, r)
		if hookErr != nil {
			return out, hookErr
		}
		r = projected
		v = hookView(v, r)
		registered := r.CreatedAt
		public := PublicView{View: v, RegisteredAt: &registered}
		if v.Receipt != nil {
			public.ProcessStartedAt = v.Receipt.StartedAt
		}
		o, readErr := loadObservation(state, r)
		if readErr != nil && !os.IsNotExist(readErr) {
			public.ObservationError = core.Failure("conflict", "provider observation requires reconciliation")
			public.observationFailed()
		}
		if o != nil && readErr == nil {
			fresh := o.Fresh(time.Now().UTC())
			if v.Lifecycle == "exited" {
				fresh.Attention, fresh.Freshness = "unknown", "process_exited"
			}
			public.applyObservation(fresh)
		}
		out = append(out, public)
	}
	slices.SortFunc(out, func(a, b PublicView) int {
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

func GetExisting(ctx context.Context, scope core.ReadScope, id string) (PublicView, error) {
	if !runtimepath.ValidID(id) || !slices.Contains(scope.ManagedIDs, id) {
		return PublicView{}, core.Failure("denied", "agent is outside read scope")
	}
	scope.ManagedIDs = []string{id}
	views, err := ReadExisting(ctx, scope)
	if err != nil {
		return PublicView{}, err
	}
	if len(views) != 1 {
		return PublicView{}, core.Failure("not_found", "scoped agent is unavailable")
	}
	return views[0], nil
}

// WaitExisting waits for evidence only; it does not restart a coordinator,
// retry a launch, create state, or interpret disappearance as process success.
func WaitExisting(ctx context.Context, scope core.ReadScope, id, until string, timeout time.Duration) (PublicView, error) {
	return WaitForObservation(ctx, func(ctx context.Context) (PublicView, error) { return GetExisting(ctx, scope, id) }, until, timeout)
}

// WaitForObservation centralizes condition/deadline semantics while allowing
// the caller's scoped reader to revalidate repository/instance pins each poll.
func WaitForObservation(ctx context.Context, read func(context.Context) (PublicView, error), until string, timeout time.Duration) (PublicView, error) {
	if read == nil {
		return PublicView{}, core.Failure("invalid_config", "missing observation reader")
	}
	if timeout <= 0 || timeout > time.Minute {
		return PublicView{}, core.Failure("invalid_usage", "read wait timeout must be greater than zero and at most 60s")
	}
	if until != "exited" && until != "idle" && until != "needs_input" && until != "turn_complete" {
		return PublicView{}, core.Failure("invalid_usage", "unsupported read wait condition")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var latest PublicView
	for {
		v, err := read(ctx)
		if err != nil {
			return latest, err
		}
		latest = v
		if until == "exited" && !v.Supervised {
			return v, core.Failure("unsupported", "unsupervised registrations have no exit receipt")
		}
		if until == "exited" && v.Receipt != nil {
			return v, nil
		}
		if until != "exited" {
			if v.Hooks == nil {
				return v, core.Failure("unsupported", "attention wait requires configured observations")
			}
			if until == "turn_complete" && v.ProviderObservation != nil && v.ProviderObservation.Freshness == "fresh" && v.ProviderObservation.TurnOutcome == "completed" && v.ProviderObservation.TurnCompleteAt != nil {
				return v, nil
			}
			if until != "turn_complete" && v.Attention == until {
				return v, nil
			}
			if v.Lifecycle == "exited" {
				return v, core.Failure("operation_failed", "process exited before the requested observation")
			}
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return latest, ctx.Err()
		case <-timer.C:
		}
	}
}
