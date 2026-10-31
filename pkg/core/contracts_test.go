package core

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
	"unicode/utf8"
)

func TestEnvelopeGoldens(t *testing.T) {
	stamp := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	zero := 0
	cases := map[string]Envelope{
		"success": {1, struct {
			Session Session `json:"session"`
		}{Session{Locator: SessionLocator{HostID: "0123456789abcdef0123456789abcdef", Backend: "tmux", Name: "sample"}, BackendID: String("$1"), ProcessState: "unknown", ClientState: "detached", AttachedClients: &zero, ObservedAt: stamp, Source: "tmux session listing", Capabilities: Capabilities{}}}, []*OperationError{}, false},
		"raw": {1, Snapshot{Locator: SessionLocator{HostID: "0123456789abcdef0123456789abcdef", Backend: "native", Name: "sample"}, Type: "screen_snapshot", Format: "raw", Content: "YWJj", Encoding: String("base64"), Lines: 1, Bytes: 3, ObservedAt: stamp}, []*OperationError{}, false},
		"empty": {1, struct {
			Sessions []Session `json:"sessions"`
		}{[]Session{}}, []*OperationError{}, false},
		"error": {1, nil, []*OperationError{Failure("not_found", "exact session not found")}, false},
		"partial": {1, struct {
			Sessions []Session `json:"sessions"`
		}{[]Session{}}, []*OperationError{Scope(Failure("backend_unavailable", "backend executable unavailable"), "screen")}, true},
	}
	for name, e := range cases {
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile("testdata/" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			var a, b any
			if json.Unmarshal(want, &a) != nil || json.Unmarshal(got, &b) != nil || !reflect.DeepEqual(a, b) {
				t.Fatalf("golden mismatch\ngot %s\nwant %s", got, want)
			}
		})
	}
}
func TestExitPrecedence(t *testing.T) {
	cases := []struct {
		codes   []string
		partial bool
		want    int
	}{
		{nil, false, 0}, {[]string{"internal"}, false, 1}, {[]string{"invalid_usage"}, false, 2}, {[]string{"not_found"}, false, 3}, {[]string{"unsupported"}, false, 4}, {[]string{"ambiguous_target"}, false, 5}, {[]string{"timeout"}, false, 6}, {[]string{"backend_unavailable"}, false, 7}, {[]string{"denied"}, false, 8}, {[]string{"cancelled"}, false, 130},
		{[]string{"operation_failed", "not_found", "unsupported", "backend_unavailable", "timeout", "conflict", "denied", "invalid_target"}, false, 2},
		{[]string{"timeout", "denied"}, false, 8}, {[]string{"unsupported", "backend_unavailable"}, false, 7}, {[]string{"denied"}, true, 7}, {[]string{"cancelled"}, true, 130},
	}
	for _, c := range cases {
		es := []*OperationError{}
		for _, code := range c.codes {
			es = append(es, Failure(code, "fixture"))
		}
		if got := ExitCode(es, c.partial); got != c.want {
			t.Errorf("%v partial=%v: got %d want %d", c.codes, c.partial, got, c.want)
		}
	}
}
func TestSafePlainAndOutputBounds(t *testing.T) {
	cases := map[string]string{
		"a\x1b[31mred\x1b[0mz":            "aredz",
		"before\x1b]52;c;secret\x07after": "beforeafter",
		"before\x1bPmalicious\x1b\\after": "beforeafter",
		"before\x1b_hidden\x1b\\after":    "beforeafter",
		"before\x1b]unterminated":         "before",
		"a\x9b31mb\x9b0mc":                "abc",
		"a\u009dclipboard\u009cz":         "az",
		"a\x00\x08\x7f\r\nb\tλ雪":          "a\nb\tλ雪",
		"a\xe2\x82":                       "a",
	}
	for in, want := range cases {
		if got := Plain([]byte(in)); got != want {
			t.Errorf("%q => %q want %q", in, got, want)
		}
	}
	o := CaptureOptions{Lines: 2, Bytes: 5}
	s := MakeSnapshot("tmux", "s", []byte("one\ntwo\nλ雪\n"), o, false)
	if s.Content != "two\n" || s.Bytes != 4 || s.Lines != 1 || !s.Truncated {
		t.Fatalf("UTF-8/line cap: %+v", s)
	}
	raw := MakeSnapshot("native", "s", []byte{27, '[', 'm', 255}, CaptureOptions{Lines: 2, Bytes: 4, Raw: true}, false)
	if raw.Content != "G1tt/w==" || raw.Encoding == nil || *raw.Encoding != "base64" || raw.Bytes != 4 {
		t.Fatalf("raw encoding: %+v", raw)
	}
	for _, o := range []CaptureOptions{{0, 1, false, false}, {1, 0, false, false}, {10001, 1, false, false}, {1, MaxBytes + 1, false, false}} {
		if o.Validate() == nil {
			t.Errorf("accepted invalid bounds %+v", o)
		}
	}
}

func FuzzPlain(f *testing.F) {
	for _, seed := range [][]byte{[]byte("plain λ雪"), []byte("a\x1b]52;c;secret\x1b\\z"), {0x9b, '3', '1', 'm', 'x'}, {0xe2, 0x82}, []byte("\x1bPunterminated")} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		out := Plain(data)
		if !utf8.ValidString(out) || len(out) > len(data) {
			t.Fatal("invalid or expanded UTF-8 output")
		}
		for _, r := range out {
			if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
				t.Fatalf("terminal control survived: %U", r)
			}
		}
	})
}
