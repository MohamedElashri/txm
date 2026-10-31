package dashboard

import (
	"github.com/MohamedElashri/txm/pkg/core"
	"strings"
	"testing"
	"time"
)

func TestDenyOnlyAncestryParsingAndUnavailableContext(t *testing.T) {
	parent, tty, ok := procContext("123 (name with ) brackets) S 42 43 44 34817 99")
	if !ok || parent != 42 || tty != "34817" {
		t.Fatal(parent, tty, ok)
	}
	if _, _, ok := procContext("invalid"); ok {
		t.Fatal("invalid proc stat accepted")
	}
	id := strings.Repeat("a", 32)
	for _, args := range [][]string{{"/tmp/renamed", "server", "--instance", id, "name"}, {"txm", "server", "--instance=" + id, "name"}} {
		if serverInstance(args) != id {
			t.Fatal("framed server not recognized")
		}
	}
	if serverInstance([]string{"txm", "agent", "server", "--instance", id}) != "" {
		t.Fatal("unrelated argv accepted")
	}
	for _, args := range [][]string{{"txm", "server", "--instance", "../bad"}, {"txm", "server", "name"}} {
		if serverInstance(args) != UnverifiedNativeContext {
			t.Fatal("unverified server context granted an action")
		}
	}
	o := options()
	o.CurrentNativeID = UnverifiedNativeContext
	m := NewModel(o)
	m.ApplyBackend("native", []core.Session{session("self", "1")}, nil, time.Now())
	row := m.Rows(time.Now())[0]
	if row.Action("attach") == nil || row.Lifecycle() != "running" {
		t.Fatal("context uncertainty granted an action or changed lifecycle")
	}
	o.CurrentNativeID = "1"
	m = NewModel(o)
	m.ApplyBackend("native", []core.Session{session("self", "1")}, nil, time.Now())
	if !m.Rows(time.Now())[0].Current {
		t.Fatal("current instance not disabled")
	}
}
