package agent

import (
	"os"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

// PublicView extends public JSON without altering strict v1 coordinator IPC.
type PublicView struct {
	RegisteredAt     *time.Time `json:"registered_at,omitempty"`
	ProcessStartedAt *time.Time `json:"process_started_at,omitempty"`
	View
	ProviderObservation *ObservationView     `json:"provider_observation"`
	ObservationError    *core.OperationError `json:"observation_error,omitempty"`
}

func (v *PublicView) observationFailed() {
	v.Attention = "unknown"
	v.AttentionSource = "provider observation requires reconciliation"
	v.TurnCompleteAt = nil
}
func (v *PublicView) applyObservation(o ObservationView) {
	v.ProviderObservation = &o
	v.Attention = o.Attention
	if o.Attention == "permission_requested" {
		v.Attention = "unknown"
	}
}

// PublicViews enriches only the supplied identities. It never creates state,
// probes providers/backends, starts a coordinator, or repairs a sidecar.
func PublicViews(views []View) []PublicView {
	out := make([]PublicView, len(views))
	needs := false
	for i, v := range views {
		out[i].View = v
		if v.Receipt != nil {
			out[i].ProcessStartedAt = v.Receipt.StartedAt
		}
		needs = needs || v.Hooks != nil
	}
	if !needs {
		return out
	}
	failure := func(err error) []PublicView {
		if !os.IsNotExist(err) {
			for i := range out {
				if out[i].Hooks != nil {
					out[i].ObservationError = core.Failure("invalid_config", "provider observation unavailable or unsafe")
					out[i].observationFailed()
				}
			}
		}
		return out
	}
	root, err := runtimepath.OpenExistingState()
	if err != nil {
		return failure(err)
	}
	defer func() { _ = root.Close() }()
	state, err := root.ChildExisting("agents")
	if err != nil {
		return failure(err)
	}
	defer func() { _ = state.Close() }()
	registry, err := LoadRegistry(state, views[0].Locator.HostID)
	if err != nil {
		return failure(err)
	}
	records := make(map[string]Record, len(registry.Agents))
	for _, r := range registry.Agents {
		records[r.ID] = r
	}
	for i, v := range views {
		if v.Hooks == nil {
			continue
		}
		r, ok := records[v.ID]
		if ok {
			projected, err := projectExternalHook(state, r)
			if err != nil {
				out[i].ObservationError = core.Failure("conflict", "provider association requires reconciliation")
				out[i].observationFailed()
				continue
			}
			r = projected
		}
		if !ok || r.Hook == nil || r.Locator.HostID != v.Locator.HostID || r.Hook.Generation != v.Hooks.Generation || r.Hook.InstanceID != v.Hooks.InstanceID || r.Hook.Sequence != v.Hooks.Sequence {
			out[i].ObservationError = core.Failure("conflict", "provider observation changed; query again")
			out[i].observationFailed()
			continue
		}
		o, err := loadObservation(state, r)
		if err != nil {
			if !os.IsNotExist(err) {
				out[i].ObservationError = core.Failure("conflict", "provider observation requires reconciliation")
				out[i].observationFailed()
			}
			continue
		}
		if o != nil {
			fresh := o.Fresh(time.Now().UTC())
			if v.Lifecycle == "exited" {
				fresh.Attention, fresh.Freshness = "unknown", "process_exited"
			}
			out[i].applyObservation(fresh)
		}
	}
	return out
}
