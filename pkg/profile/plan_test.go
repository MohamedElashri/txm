package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/MohamedElashri/txm/pkg/core"
)

func TestCompileCwdDefaultsNamesAndPrefix(t *testing.T) {
	base := t.TempDir()
	cwd := t.TempDir()
	sub := filepath.Join(base, "cwd with spaces")
	if e := os.Mkdir(sub, 0700); e != nil {
		t.Fatal(e)
	}
	spec := &config.Profile{Name: "sample", Backend: "tmux", SourceDir: base, Windows: []config.Window{{Panes: []config.Pane{{Dir: "cwd with spaces", Command: "private synthetic"}, {Split: "h"}}}, {}}}
	plan, e := Compile(spec, "tmux", "pre_", cwd)
	if e != nil {
		t.Fatal(e)
	}
	if plan.Name != "pre_sample" || plan.Windows[0].Name != "window-1" || plan.Windows[1].Name != "window-2" || plan.Windows[0].Panes[0].Dir != sub || plan.Windows[0].Panes[1].Dir != cwd {
		t.Fatalf("wrong compiled targets/cwd: %+v", plan)
	}
	spec.Name = "pre_sample"
	plan, e = Compile(spec, "tmux", "pre_", cwd)
	if e != nil || plan.Name != "pre_sample" {
		t.Fatal("prefix applied twice", e)
	}
}
func TestProfileValidationBeforeApply(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		spec          config.Profile
		backend, code string
	}{
		{config.Profile{}, "tmux", "invalid_profile"},
		{config.Profile{Name: "bad name"}, "tmux", "invalid_profile"},
		{config.Profile{Name: "s", Backend: "native"}, "tmux", "invalid_profile"},
		{config.Profile{Name: "s", Windows: []config.Window{{Name: "a"}, {Name: "a"}}}, "tmux", "invalid_profile"},
		{config.Profile{Name: "s", Windows: []config.Window{{Layout: "custom-secret"}}}, "tmux", "invalid_profile"},
		{config.Profile{Name: "s", Windows: []config.Window{{Panes: []config.Pane{{Split: "h"}}}}}, "tmux", "invalid_profile"},
		{config.Profile{Name: "s", Windows: []config.Window{{Panes: []config.Pane{{}, {Split: "x"}}}}}, "tmux", "invalid_profile"},
		{config.Profile{Name: "s", Windows: []config.Window{{Panes: []config.Pane{{Dir: "missing"}}}}}, "tmux", "invalid_profile"},
		{config.Profile{Name: "s", Windows: []config.Window{{Panes: []config.Pane{{Command: "a\x00b"}}}}}, "tmux", "invalid_profile"},
		{config.Profile{Name: "s", Windows: []config.Window{{Panes: []config.Pane{{Command: strings.Repeat("x", 16384)}}}}}, "native", "invalid_profile"},
		{config.Profile{Name: "s", Windows: []config.Window{{Name: "named"}}}, "native", "unsupported"},
		{config.Profile{Name: "s", Windows: []config.Window{{Layout: "tiled"}}}, "native", "unsupported"},
		{config.Profile{Name: "s", Windows: []config.Window{{}, {}}}, "native", "unsupported"},
		{config.Profile{Name: "s"}, "screen", "unsupported"},
		{config.Profile{Name: "s"}, "zellij", "unsupported"},
	}
	for _, c := range cases {
		c.spec.SourceDir = dir
		_, e := Compile(&c.spec, c.backend, "", dir)
		if e == nil || core.Normalize(e).Code != c.code {
			t.Errorf("%+v on %s: want %s, got %v", c.spec, c.backend, c.code, e)
		}
	}
	for _, backend := range []string{"tmux", "native"} {
		if _, e := Compile(&config.Profile{Name: "s"}, backend, "", dir); e != nil {
			t.Fatal("default single terminal rejected", e)
		}
	}
}
