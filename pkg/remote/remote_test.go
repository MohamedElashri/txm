package remote

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/MohamedElashri/txm/pkg/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testHost = "0123456789abcdef0123456789abcdef"

func TestCommandRoundTripAndRejections(t *testing.T) {
	h := Host{Alias: "fixture", HostID: testHost, Executable: "/safe/txm"}
	q := Request{Schema: 1, Protocol: 1, Op: "attach", HostID: testHost, Backend: "tmux", BackendID: "$9", Name: "quote '; $(touch /tmp/never)\n雪"}
	s, e := Command(h, q)
	if e != nil {
		t.Fatal(e)
	}
	fields := strings.Fields(s)
	if len(fields) != 5 || strings.Contains(s, "touch") || strings.Contains(s, "$") {
		t.Fatal("shell text escaped encoding", s)
	}
	got, e := Decode(fields[4])
	if e != nil || got != q {
		t.Fatal(got, e)
	}
	for _, alias := range []string{"-oProxyCommand=x", "user@host", "host:22", "a b", "a;id", "ssh://x", ""} {
		if validHost(Host{alias, "", "txm"}) {
			t.Fatal(alias)
		}
	}
	for _, exe := range []string{"txm;id", "./txm", "/space path/txm", "/tmp/$(id)"} {
		if validHost(Host{"safe", "", exe}) {
			t.Fatal(exe)
		}
	}
	bad := base64.RawURLEncoding.EncodeToString([]byte(`{"schema_version":1,"schema_version":1}`))
	if _, e = Decode(bad); e == nil {
		t.Fatal("duplicate keys accepted")
	}
	q.Protocol = 2
	if _, e = Encode(q); core.Normalize(e).Code != "unsupported_version" {
		t.Fatal(e)
	}
}
func TestResponseIdentityAndVersion(t *testing.T) {
	q := Request{Schema: 1, Protocol: 1, Op: "list", HostID: testHost, Backend: "tmux"}
	s := core.Session{Locator: core.SessionLocator{HostID: testHost, Backend: "tmux", Name: "one"}, BackendID: core.String("$1")}
	d := Data{1, testHost, []core.Session{s}}
	if e := validateData(d, q); e != nil {
		t.Fatal(e)
	}
	d.Sessions = append(d.Sessions, s)
	if validateData(d, q) == nil {
		t.Fatal("duplicate identity accepted")
	}
	d.Sessions = []core.Session{}
	d.HostID = strings.Repeat("a", 32)
	if core.Normalize(validateData(d, q)).Code != "conflict" {
		t.Fatal("host pin ignored")
	}
	d.HostID = testHost
	d.Protocol = 2
	if core.Normalize(validateData(d, q)).Code != "unsupported_version" {
		t.Fatal("protocol ignored")
	}
}
func TestAllowlistAtomicIdentity(t *testing.T) {
	home, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, e := Allow(ctx, Host{"one", testHost, "txm"}); e != nil {
		t.Fatal(e)
	}
	if _, e := Allow(ctx, Host{"two", testHost, "txm"}); e == nil {
		t.Fatal("identity collision accepted")
	}
	hs, e := Hosts(ctx)
	if e != nil || len(hs) != 1 {
		t.Fatal(hs, e)
	}
	if _, e = Pin(ctx, "one", strings.Repeat("a", 32)); e != nil {
		t.Fatal(e)
	}
	if _, e = Forget(ctx, "one"); e != nil {
		t.Fatal(e)
	}
}
func TestSSHBoundedFailureAndCancellation(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("POSIX fake SSH executable")
	}
	dir, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	exe := filepath.Join(dir, "ssh")
	t.Setenv("PATH", dir)
	q := Request{Schema: 1, Protocol: 1, Op: "probe"}
	h := Host{"fixture", "", "txm"}
	env := core.Envelope{SchemaVersion: 1, Data: Data{1, testHost, []core.Session{}}, Errors: []*core.OperationError{}}
	b, _ := json.Marshal(env)
	if e := os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s' '"+string(b)+"'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	d, e := Query(context.Background(), h, q, time.Second)
	if e != nil || d.HostID != testHost {
		t.Fatal(d, e)
	}
	if e = os.WriteFile(exe, []byte("#!/bin/sh\nexec /bin/sleep 10\n"), 0700); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, e = Query(ctx, h, q, time.Second)
	if core.Normalize(e).Code != "cancelled" || time.Since(start) > time.Second {
		t.Fatal(e)
	}
	_, e = Query(context.Background(), h, q, time.Second)
	if core.Normalize(e).Code != "timeout" {
		t.Fatal(e)
	}
	if e = os.WriteFile(exe, []byte("#!/bin/sh\nexit 255\n"), 0700); e != nil {
		t.Fatal(e)
	}
	_, e = Query(context.Background(), h, q, time.Second)
	if core.Normalize(e).Code != "backend_unavailable" {
		t.Fatal(e)
	}
}
func FuzzDecode(f *testing.F) {
	q := Request{Schema: 1, Protocol: 1, Op: "probe"}
	s, _ := Encode(q)
	f.Add(s)
	f.Add("!")
	f.Fuzz(func(t *testing.T, s string) {
		q, e := Decode(s)
		if e == nil {
			if q.Validate() != nil {
				t.Fatal("invalid decoded request")
			}
			if _, e = Encode(q); e != nil {
				t.Fatal(e)
			}
		}
	})
}

// A missing/null partial flag must not be confused with a verified complete
// response. Zero-valued Go bools otherwise erase that wire distinction.
func TestSSHEnvelopeRequiredFields(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("POSIX fake SSH executable")
	}
	dir, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	t.Setenv("PATH", dir)
	exe := filepath.Join(dir, "ssh")
	valid := `{"schema_version":1,"data":{"protocol_version":1,"host_id":"` + testHost + `","sessions":[]},"errors":[],"partial":false}`
	cases := []struct{ name, payload, code string }{
		{"complete", valid, ""},
		{"missing partial", strings.Replace(valid, `,"partial":false`, "", 1), "protocol_error"},
		{"null partial", strings.Replace(valid, `"partial":false`, `"partial":null`, 1), "protocol_error"},
		{"true partial", strings.Replace(valid, `"partial":false`, `"partial":true`, 1), "protocol_error"},
		{"missing errors", strings.Replace(valid, `,"errors":[]`, "", 1), "protocol_error"},
		{"null errors", strings.Replace(valid, `"errors":[]`, `"errors":null`, 1), "protocol_error"},
		{"missing data in error", `{"schema_version":1,"errors":[{"code":"conflict"}],"partial":false}`, "protocol_error"},
		{"complete error", `{"schema_version":1,"data":null,"errors":[{"code":"conflict"}],"partial":false}`, "conflict"},
		{"schema mismatch", strings.Replace(valid, `"schema_version":1`, `"schema_version":2`, 1), "unsupported_version"},
		{"unknown key", strings.Replace(valid, `"partial":false`, `"partial":false,"unexpected":true`, 1), "protocol_error"},
		{"duplicate flag", strings.Replace(valid, `"partial":false`, `"partial":false,"partial":false`, 1), "protocol_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if e := os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s' '"+tc.payload+"'\n"), 0700); e != nil {
				t.Fatal(e)
			}
			_, e := Query(context.Background(), Host{"fixture", "", "txm"}, Request{Schema: 1, Protocol: 1, Op: "probe"}, time.Second)
			if tc.code == "" {
				if e != nil {
					t.Fatal(e)
				}
			} else if e == nil || core.Normalize(e).Code != tc.code {
				t.Fatalf("want %s, got %v", tc.code, e)
			}
		})
	}
}
