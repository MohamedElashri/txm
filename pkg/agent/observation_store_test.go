package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

func TestObservationSidecarDoesNotChangeV1AndRejectsTornProjection(t *testing.T) {
	if !runtimepath.Supported() {
		t.Skip("private Unix storage")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(base, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", base)
	host, err := runtimepath.HostID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	root, err := runtimepath.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	state, err := root.Child("agents")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = state.Close() }()
	id := observationIdentity()
	now := time.Now().UTC()
	o, err := ReduceObservation(nil, id, ProviderEvent{Kind: "UserPromptSubmit", SessionID: "thread", TurnID: "turn"}, 1, now, now)
	if err != nil {
		t.Fatal(err)
	}
	r := Record{ID: id.ManagedID, ManagedID: &id.ManagedID, InstanceID: &id.InstanceID, Supervised: true, CreatedAt: now, Locator: core.SessionLocator{HostID: host, Backend: "native", Name: "fixture"}, Hook: &HookState{Provider: id.Provider, Version: id.Version, InstanceID: id.InstanceID, Generation: id.Generation, SessionID: o.SessionID, Sequence: 1, Attention: "working", ObservedAt: &now}}
	if err = saveRegistry(state, Registry{SchemaVersion: 1, HostID: host, Agents: []Record{r}}); err != nil {
		t.Fatal(err)
	}
	before, err := state.ReadPrivate("registry.json", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(o)
	if err = state.WritePrivate(observationName(id.Generation), encoded, false); err != nil {
		t.Fatal(err)
	}
	if _, err = loadObservation(state, r); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadRegistry(state, host); err != nil {
		t.Fatal("sidecar changed v1 compatibility", err)
	}
	after, _ := state.ReadPrivate("registry.json", 1<<20)
	if string(before) != string(after) || strings.Contains(string(after), "turn_id") {
		t.Fatal("canonical metadata embedded in v1 registry")
	}
	o.Sequence = 2
	encoded, _ = json.Marshal(o)
	if err = state.WritePrivate(observationName(id.Generation), encoded, true); err != nil {
		t.Fatal(err)
	}
	if _, err = loadObservation(state, r); err == nil {
		t.Fatal("torn publication accepted")
	}
	legacy := hookView(View{ID: r.ID, Locator: r.Locator, Lifecycle: "running"}, r)
	public := PublicViews([]View{legacy})
	if len(public) != 1 || public[0].ObservationError == nil || public[0].Attention != "unknown" {
		t.Fatal("torn sidecar retained actionable legacy attention", public)
	}
	o.Sequence = 1
	o.Attention = "invented"
	encoded, _ = json.Marshal(o)
	if err = state.WritePrivate(observationName(id.Generation), encoded, true); err != nil {
		t.Fatal(err)
	}
	if _, err = loadObservation(state, r); err == nil {
		t.Fatal("invalid state accepted")
	}
	if err = state.RemovePrivate(observationName(id.Generation)); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadRegistry(state, host); err != nil {
		t.Fatal("old records now require sidecar", err)
	}
}
