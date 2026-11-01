package core

import (
	"path/filepath"
	"regexp"
	"slices"
	"sync"
)

var opaqueID = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[a-f0-9]{32}$`) })

// ReadScope is immutable caller policy, never a model-supplied tool argument.
// When both IDs and repositories are supplied they intersect, not broaden one
// another. Repository provenance must be positively verified by the task layer.
type ReadScope struct {
	HostID       string   `json:"host_id"`
	Backends     []string `json:"backends"`
	ManagedIDs   []string `json:"managed_ids"`
	Repositories []string `json:"repositories"`
}

func (s ReadScope) Empty() bool {
	return len(s.Backends) == 0 || (len(s.ManagedIDs) == 0 && len(s.Repositories) == 0)
}

func (s ReadScope) Validate() error {
	bad := func() error { return Failure("invalid_config", "invalid bounded read scope") }
	if s.HostID != "" && !opaqueID().MatchString(s.HostID) {
		return bad()
	}
	if len(s.Backends) > 2 || len(s.ManagedIDs) > 256 || len(s.Repositories) > 128 {
		return bad()
	}
	for _, b := range s.Backends {
		if b != "native" && b != "tmux" {
			return bad()
		}
	}
	for _, id := range s.ManagedIDs {
		if !opaqueID().MatchString(id) {
			return bad()
		}
	}
	for _, repo := range s.Repositories {
		if !filepath.IsAbs(repo) || filepath.Clean(repo) != repo {
			return bad()
		}
	}
	for _, values := range [][]string{s.Backends, s.ManagedIDs, s.Repositories} {
		seen := map[string]bool{}
		for _, v := range values {
			if seen[v] {
				return bad()
			}
			seen[v] = true
		}
	}
	return nil
}

func (s ReadScope) Allows(host, backend, id, verifiedRepository string) bool {
	return !s.Empty() && (s.HostID == "" || s.HostID == host) && slices.Contains(s.Backends, backend) &&
		(len(s.ManagedIDs) == 0 || slices.Contains(s.ManagedIDs, id)) &&
		(len(s.Repositories) == 0 || slices.Contains(s.Repositories, verifiedRepository))
}
