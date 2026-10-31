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

func TestNewProviderAssociationLeavesStrictV1RegistryReadable(t *testing.T) {
	if !runtimepath.Supported() {
		t.Skip("private Unix storage")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(base, 0700)
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
	id, instance, generation := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 32)
	r := Record{ID: id, ManagedID: &id, InstanceID: &instance, Supervised: true, CreatedAt: time.Now().UTC(), Locator: core.SessionLocator{HostID: host, Backend: "native", Name: "fixture"}}
	if err = saveRegistry(state, Registry{SchemaVersion: 1, HostID: host, Agents: []Record{r}}); err != nil {
		t.Fatal(err)
	}
	before, _ := state.ReadPrivate("registry.json", 1<<20)
	virtual := r
	virtual.Hook = &HookState{Provider: "codex", Version: liveCodexVersion, InstanceID: instance, Generation: generation, Attention: "unknown"}
	if virtual.Hook.validate(r) {
		t.Fatal("new version admitted into strict v1 record")
	}
	if err = saveExternalHook(state, virtual); err != nil {
		t.Fatal(err)
	}
	after, _ := state.ReadPrivate("registry.json", 1<<20)
	if string(before) != string(after) {
		t.Fatal("association rewrote v1 registry")
	}
	loaded, err := LoadRegistry(state, host)
	if err != nil || loaded.Agents[0].Hook != nil {
		t.Fatal("v1 registry changed", err)
	}
	projected, err := projectExternalHook(state, r)
	if err != nil || projected.Hook == nil || projected.Hook.Version != liveCodexVersion || r.Hook != nil {
		t.Fatal("external projection mutated base", err)
	}
	// A different supervisor instance cannot inherit the old association.
	other := strings.Repeat("d", 32)
	changed := r
	changed.InstanceID = &other
	if _, err = projectExternalHook(state, changed); err == nil {
		t.Fatal("instance replacement borrowed association")
	}
	// An explicit legacy association revokes this projection immediately.
	legacy := r
	legacy.Hook = &HookState{Provider: "generic", Version: "1", InstanceID: instance, Generation: strings.Repeat("e", 32), Attention: "unknown"}
	projected, err = projectExternalHook(state, legacy)
	if err != nil || projected.Hook.Provider != "generic" {
		t.Fatal("legacy rotation did not revoke", err)
	}
	// Copying a sidecar to another namespace must fail without repairing it.
	data, _ := state.ReadPrivate(externalAssociationName(id), 8192)
	var stored externalAssociation
	_ = json.Unmarshal(data, &stored)
	stored.StateRoot = "/another/namespace"
	data, _ = json.Marshal(stored)
	if err = state.WritePrivate(externalAssociationName(id), data, true); err != nil {
		t.Fatal(err)
	}
	if _, err = projectExternalHook(state, r); err == nil {
		t.Fatal("foreign namespace association accepted")
	}
}
