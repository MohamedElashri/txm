package supervisor

import (
	"encoding/json"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"os"
	"path/filepath"
	"testing"
)

func validManifest() Manifest {
	return Manifest{1, "11111111111111111111111111111111", "22222222222222222222222222222222", "native", []string{"/bin/sh", "-c", "private argv"}, "/tmp", []string{"PATH"}, ReceiptName("22222222222222222222222222222222"), nil}
}
func TestManifestValidation(t *testing.T) {
	m := validManifest()
	if e := m.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Manifest){
		func(m *Manifest) { m.Argv = nil }, func(m *Manifest) { m.Argv = []string{"relative"} }, func(m *Manifest) { m.Argv = []string{"/bin/sh", "x\x00y"} }, func(m *Manifest) { m.Cwd = "relative" }, func(m *Manifest) { m.EnvNames = []string{"A=secret"} }, func(m *Manifest) { m.EnvNames = []string{"PATH", "PATH"} }, func(m *Manifest) { m.Receipt = "../receipt" }, func(m *Manifest) { m.ManagedID = m.InstanceID }, func(m *Manifest) { m.Backend = "screen" },
	} {
		m := validManifest()
		change(&m)
		if m.Validate() == nil {
			t.Fatal("invalid manifest accepted", m)
		}
	}
}
func TestReceiptIdentityOutcomesAndImmutability(t *testing.T) {
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
	r, e := State()
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = r.Close() }()
	m := validManifest()
	now := Now()
	code := 42
	v := Receipt{1, m.ManagedID, m.InstanceID, "native", &now, now, "exited", &code, nil}
	data, _ := json.Marshal(v)
	if e = r.WritePrivate(m.Receipt, data, false); e != nil {
		t.Fatal(e)
	}
	if got, e := ReadReceipt(r, m.ManagedID, m.InstanceID, "native"); e != nil || *got.ExitCode != 42 {
		t.Fatal(got, e)
	}
	if _, e = ReadReceipt(r, "33333333333333333333333333333333", m.InstanceID, "native"); e == nil {
		t.Fatal("accepted unrelated receipt")
	}
	if r.WritePrivate(m.Receipt, []byte("{}"), false) == nil {
		t.Fatal("receipt overwritten")
	}
	v.Outcome = "start_failed"
	data, _ = json.Marshal(v)
	if e = r.WritePrivate(m.Receipt, data, true); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadReceipt(r, m.ManagedID, m.InstanceID, "native"); e == nil {
		t.Fatal("invented exit for failed exec")
	}
}
