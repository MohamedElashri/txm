package backend

import (
	"context"
	"testing"
)

func TestAppleScreenListingExitOneRequiresCompleteRecognizedOutput(t *testing.T) {
	for _, test := range []struct {
		body string
		ok   bool
	}{
		{"printf 'There are screens on:\\n\\t123.foo\\t(Detached)\\n\\t456.foobar\\t(Attached)\\n2 Sockets in /private/fixture.\\n'; exit 1", true},
		{"printf '\\t123.foo\\t(Detached)\\n2 Sockets in /private/fixture.\\n'; exit 1", false},
		{"printf '\\t123.foo\\t(Detached)\\n'; exit 1", false},
		{"printf 'No Sockets found in /private/fixture.\\n'; exit 1", true},
		{"printf 'No Sockets found in /private/fixture.\\n'; printf 'failed' >&2; exit 1", false},
		{"printf '\\t123.foo\\t(Detached)\\n1 Socket in /private/fixture.\\n'; printf 'failed' >&2; exit 1", false},
		{"printf '\\t123.foo\\t(Detached)\\n1 Socket in /private/fixture.\\n'; exit 2", false},
	} {
		dir := t.TempDir()
		executable(t, dir, "screen", test.body)
		t.Setenv("PATH", dir)
		rows, err := NewScreenBackend().Sessions(context.Background())
		if (err == nil) != test.ok {
			t.Fatal(test.body, rows, err)
		}
	}
}

func TestAppleScreenLegacyVersionBannerExitOne(t *testing.T) {
	for _, banner := range []string{"Screen version 4.00.03 (FAU) 23-Oct-06", "Screen version 4.00.03 failed", "unknown"} {
		dir := t.TempDir()
		executable(t, dir, "screen", "printf '%s\\n' '"+banner+"'; exit 1")
		t.Setenv("PATH", dir)
		capability, err := NewScreenBackend().BackendCapabilities(context.Background())
		want := banner == "Screen version 4.00.03 (FAU) 23-Oct-06"
		if (err == nil) != want || (want && (capability.Version == nil || capability.Capabilities["session_lifecycle"].Level != "degraded")) {
			t.Fatal(capability, err)
		}
	}
}
