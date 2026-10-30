package agent

import (
	"encoding/json"
	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryStrictIdentityAndNoReplayIntent(t *testing.T) {
	if !runtimepath.Supported() {
		t.Skip("private Unix state")
	}
	base, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	if e := os.Chmod(base, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("XDG_STATE_HOME", base)
	state, e := supervisor.State()
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = state.Close() }()
	host := "11111111111111111111111111111111"
	id := "22222222222222222222222222222222"
	instance := "33333333333333333333333333333333"
	reg := Registry{1, host, []Record{{ID: id, ManagedID: &id, InstanceID: &instance, Locator: core.SessionLocator{HostID: host, Backend: "native", Name: "sample"}, Supervised: true, CreatedAt: supervisor.Now()}}}
	if e = saveRegistry(state, reg); e != nil {
		t.Fatal(e)
	}
	if _, e = LoadRegistry(state, host); e != nil {
		t.Fatal("pre-launch intent rejected", e)
	}
	reg.Agents[0].Binding = backend.ManagedBinding{BackendID: instance, NativeInstance: instance}
	reg.Agents = append(reg.Agents, reg.Agents[0])
	if e = saveRegistry(state, reg); e != nil {
		t.Fatal(e)
	}
	if _, e = LoadRegistry(state, host); e == nil {
		t.Fatal("duplicate ID accepted")
	}
	for _, data := range []string{`{"schema_version":1,"host_id":"` + host + `","agents":[{"ID":"bad"}]}`, `{"schema_version":99}`, `{"schema_version":1,"schema_version":1}`} {
		if e = state.WritePrivate("registry.json", []byte(data), true); e != nil {
			t.Fatal(e)
		}
		if _, e = LoadRegistry(state, host); e == nil {
			t.Fatal("corrupt registry accepted")
		}
	}
	data, _ := json.Marshal(Registry{1, host, []Record{}})
	if e = state.WritePrivate("registry.json", data, true); e != nil {
		t.Fatal(e)
	}
	if _, e = LoadRegistry(state, "44444444444444444444444444444444"); e == nil {
		t.Fatal("host identity replaced")
	}
}
