package core

import (
	"strings"
	"testing"
)

func TestReadScopeIsDefaultDenyAndSelectorsIntersect(t *testing.T) {
	id, host := strings.Repeat("a", 32), strings.Repeat("b", 32)
	if (ReadScope{}).Allows(host, "native", id, "/repo") {
		t.Fatal("default scope allowed read")
	}
	s := ReadScope{HostID: host, Backends: []string{"native"}, ManagedIDs: []string{id}, Repositories: []string{"/repo"}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if !s.Allows(host, "native", id, "/repo") {
		t.Fatal("exact scope denied")
	}
	for _, v := range [][4]string{{host, "tmux", id, "/repo"}, {host, "native", strings.Repeat("c", 32), "/repo"}, {host, "native", id, "/other"}, {strings.Repeat("d", 32), "native", id, "/repo"}, {host, "native", id, ""}} {
		if s.Allows(v[0], v[1], v[2], v[3]) {
			t.Fatal("selector broadened scope", v)
		}
	}
	s.ManagedIDs = []string{"../bad"}
	if s.Validate() == nil {
		t.Fatal("accepted invalid exact ID")
	}
}
