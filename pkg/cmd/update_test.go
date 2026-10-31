package cmd

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/MohamedElashri/txm/pkg/core"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDevelopmentAndPrereleaseUpdateRefusedBeforeNetwork(t *testing.T) {
	originalVersion, originalClient := Version, http.DefaultClient
	t.Cleanup(func() { Version, http.DefaultClient = originalVersion, originalClient })
	for _, version := range []string{"dev", "2.0.0-rc.1", "2.0.0-beta.2"} {
		t.Run(version, func(t *testing.T) {
			Version = version
			called := false
			http.DefaultClient = &http.Client{Transport: updateTransport(func(*http.Request) (*http.Response, error) {
				called = true
				return nil, errors.New("unexpected update request")
			})}
			err := updateCmd.RunE(updateCmd, nil)
			if err == nil || core.Normalize(err).Code != "unsupported" {
				t.Fatalf("prerelease update must refuse before mutation, got %v", err)
			}
			if called {
				t.Fatal("development/prerelease build contacted the stable update endpoint")
			}
		})
	}
}

func TestStableUpdateNeverDownloadsOlderEqualOrInvalidRelease(t *testing.T) {
	originalVersion, originalClient := Version, http.DefaultClient
	t.Cleanup(func() { Version, http.DefaultClient = originalVersion, originalClient })
	for _, tc := range []struct {
		current, latest string
		invalid         bool
	}{
		{"2.0.0", "v1.3.1", false},
		{"2.0.10", "v2.0.9", false},
		{"2.10.0", "v2.9.99", false},
		{"2.0.0", "v2.0.0", false},
		{"2.0.0", "v2.0.1-rc.1", true},
		{"2.0.0", "v02.0.1", true},
		{"2.0.0", "invalid", true},
	} {
		t.Run(tc.current+"/"+tc.latest, func(t *testing.T) {
			Version = tc.current
			requests := 0
			http.DefaultClient = &http.Client{Transport: updateTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if requests != 1 || r.URL.Path != "/repos/MohamedElashri/txm/releases/latest" {
					t.Fatal("unexpected asset request", r.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"` + tc.latest + `","assets":[]}`))}, nil
			})}
			err := updateCmd.RunE(updateCmd, nil)
			if requests != 1 || (err != nil) != tc.invalid {
				t.Fatalf("requests=%d error=%v invalid=%v", requests, err, tc.invalid)
			}
		})
	}
}

func TestNewerStableUpdateReachesAssetSelection(t *testing.T) {
	originalVersion, originalClient := Version, http.DefaultClient
	t.Cleanup(func() { Version, http.DefaultClient = originalVersion, originalClient })
	Version = "2.9.0"
	http.DefaultClient = &http.Client{Transport: updateTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/repos/MohamedElashri/txm/releases/latest" {
			t.Fatal("unexpected download")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v2.10.0","assets":[]}`))}, nil
	})}
	if err := updateCmd.RunE(updateCmd, nil); err == nil || !strings.Contains(err.Error(), "no suitable binary") {
		t.Fatalf("expected newer version to reach asset selection, got %v", err)
	}
}
