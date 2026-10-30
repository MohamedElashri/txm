package cmd

import (
	"errors"
	"net/http"
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
