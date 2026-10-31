package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

func TestExistingAgentReadsDoNotInitializeMissingState(t *testing.T) {
	if !supervisor.Supported() {
		t.Skip("private Unix observations")
	}
	base := t.TempDir()
	state, runtime := filepath.Join(base, "missing-state"), filepath.Join(base, "missing-runtime")
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	id := strings.Repeat("a", 32)
	scope := core.ReadScope{Backends: []string{"native"}, ManagedIDs: []string{id}}
	if v, err := ReadExisting(context.Background(), scope); err != nil || len(v) != 0 {
		t.Fatal(v, err)
	}
	if _, err := GetExisting(context.Background(), scope, id); err == nil {
		t.Fatal("missing agent reported as present")
	}
	if _, err := GetExisting(context.Background(), scope, strings.Repeat("b", 32)); err == nil {
		t.Fatal("disallowed ID read")
	}
	for _, p := range []string{state, runtime} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatal("read initialized state", p, err)
		}
	}
}

func querySnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		var sum [32]byte
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum = sha256.Sum256(data)
		}
		out[path] = fmt.Sprintf("%v:%v:%x", info.Mode(), info.ModTime().UnixNano(), sum)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestExistingScopedReceiptReadDoesNotRewriteOrProbeExcludedState(t *testing.T) {
	if !supervisor.Supported() {
		t.Skip("private Unix observations")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(base, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", base)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(base, "absent-runtime"))
	host, err := runtimepath.HostID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	state, err := supervisor.State()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = state.Close() }()
	id, instance := strings.Repeat("a", 32), strings.Repeat("b", 32)
	other, otherInstance := strings.Repeat("c", 32), strings.Repeat("d", 32)
	now := time.Now().UTC()
	code := 0
	makeRecord := func(id, instance string) Record {
		return Record{ID: id, ManagedID: &id, InstanceID: &instance, Supervised: true, CreatedAt: now, Locator: core.SessionLocator{HostID: host, Backend: "native", Name: id}}
	}
	if err = saveRegistry(state, Registry{SchemaVersion: 1, HostID: host, Agents: []Record{makeRecord(id, instance), makeRecord(other, otherInstance)}}); err != nil {
		t.Fatal(err)
	}
	receipt := supervisor.Receipt{SchemaVersion: 1, ManagedID: id, InstanceID: instance, Backend: "native", StartedAt: &now, FinishedAt: now, Outcome: "exited", ExitCode: &code}
	data, _ := json.Marshal(receipt)
	if err = state.WritePrivate(supervisor.ReceiptName(instance), data, false); err != nil {
		t.Fatal(err)
	}
	if err = state.WritePrivate(supervisor.ReceiptName(otherInstance), []byte("PRIVATE_CORRUPT_RECEIPT"), false); err != nil {
		t.Fatal(err)
	}
	before := querySnapshot(t, base)
	scope := core.ReadScope{HostID: host, Backends: []string{"native"}, ManagedIDs: []string{id}}
	views, err := ReadExisting(context.Background(), scope)
	if err != nil || len(views) != 1 || views[0].ID != id || views[0].Receipt == nil {
		t.Fatal(views, err)
	}
	after := querySnapshot(t, base)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatal("pure query changed existing state")
	}
	if _, err = ReadPinnedExisting(context.Background(), scope, map[string]string{id: otherInstance}); err == nil {
		t.Fatal("replaced instance was probed")
	}
	if err = os.Chmod(state.Path, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadExisting(context.Background(), scope); err == nil {
		t.Fatal("unsafe store accepted")
	}
	info, _ := os.Stat(state.Path)
	if info.Mode().Perm() != 0755 {
		t.Fatal("read repaired unsafe store")
	}
}

func TestReadWaitNeverTreatsUnknownAsExit(t *testing.T) {
	v, err := WaitForObservation(context.Background(), func(context.Context) (PublicView, error) {
		return PublicView{View: View{Supervised: true, Lifecycle: "unknown"}}, nil
	}, "exited", time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) || v.Receipt != nil {
		t.Fatal("unknown became successful exit", v, err)
	}
}
