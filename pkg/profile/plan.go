// Package profile compiles declarative profiles before backend mutations.
package profile

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/MohamedElashri/txm/pkg/core"
)

type Plan struct {
	Name, Backend, Shell string
	Windows              []Window
}
type Window struct {
	Name, Layout string
	Panes        []Pane
}
type Pane struct{ Dir, Command, Split string }
type Applied struct {
	Backend, Name, SessionID, InstanceID string
	Owner                                string
	Windows                              []CreatedWindow
}
type CreatedWindow struct {
	ID    string
	Panes []string
}

func Named(name string) bool {
	return len(name) > 0 && len(name) <= 128 && regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(name)
}
func Compile(spec *config.Profile, backend, prefix, cwd string, shells ...string) (Plan, error) {
	bad := func(message string) (Plan, error) { return Plan{}, core.Failure("invalid_profile", message) }
	if spec == nil {
		return bad("missing profile")
	}
	name := spec.Name
	if prefix != "" && !strings.HasPrefix(name, prefix) {
		name = prefix + name
	}
	if !Named(spec.Name) || !Named(name) {
		return bad("profile name and prefixed name require 1..128 ASCII letters, digits, dash or underscore")
	}
	if backend != "native" && backend != "tmux" {
		return Plan{}, core.Scope(core.Failure("unsupported", "profiles require tmux or the native single terminal"), backend)
	}
	if spec.Backend != "" {
		v, e := config.ParseBackend(spec.Backend)
		if e != nil {
			return bad("invalid profile backend")
		}
		if string(v) != backend {
			return bad("profile backend conflicts with selected backend")
		}
	}
	windows := spec.Windows
	if len(windows) == 0 {
		windows = []config.Window{{}}
	}
	if len(windows) > 32 {
		return bad("profile exceeds 32 windows")
	}
	if backend == "native" && len(windows) != 1 {
		return Plan{}, core.Failure("unsupported", "native profiles require one terminal")
	}
	shell := "/bin/sh"
	if len(shells) > 0 && shells[0] != "" {
		shell = shells[0]
	}
	resolved, e := exec.LookPath(shell)
	if e != nil {
		return bad("profile shell must be an available executable")
	}
	resolved, e = filepath.Abs(resolved)
	if e != nil {
		return bad("invalid profile shell path")
	}
	result := Plan{Name: name, Backend: backend, Shell: resolved}
	seen := map[string]bool{}
	total := 0
	for i, window := range windows {
		w := Window{Name: window.Name, Layout: window.Layout}
		if backend == "native" {
			if (w.Name != "" && w.Name != "0") || w.Layout != "" {
				return Plan{}, core.Failure("unsupported", "native profiles cannot name windows or apply layouts")
			}
			w.Name = "0"
		} else {
			if w.Name == "" {
				w.Name = "window-" + strconv.Itoa(i+1)
			}
			if !Named(w.Name) {
				return bad("invalid profile window name")
			}
			if w.Layout != "" {
				switch w.Layout {
				case "even-horizontal", "even-vertical", "main-horizontal", "main-vertical", "tiled":
				default:
					return bad("unsupported profile layout name")
				}
			}
		}
		if seen[w.Name] {
			return bad("duplicate profile window name")
		}
		seen[w.Name] = true
		panes := window.Panes
		if len(panes) == 0 {
			panes = []config.Pane{{}}
		}
		total += len(panes)
		if total > 128 {
			return bad("profile exceeds 128 panes")
		}
		if backend == "native" && len(panes) != 1 {
			return Plan{}, core.Failure("unsupported", "native profiles require one pane")
		}
		for j, pane := range panes {
			if j == 0 && pane.Split != "" {
				return bad("first pane cannot request a split")
			}
			split := pane.Split
			if j > 0 && split == "" {
				split = "v"
			}
			if split != "" && split != "v" && split != "h" {
				return bad("pane split must be v or h")
			}
			limit := core.MaxBytes
			if backend == "native" {
				limit = 16383
			}
			if strings.ContainsRune(pane.Command, 0) || len(pane.Command) > limit {
				return bad("profile command contains NUL or exceeds backend input limit")
			}
			dir := pane.Dir
			if dir == "" {
				dir = cwd
			} else if !filepath.IsAbs(dir) {
				dir = filepath.Join(spec.SourceDir, dir)
			}
			absolute, e := filepath.Abs(dir)
			if e != nil {
				return bad("invalid pane cwd")
			}
			absolute, e = filepath.EvalSymlinks(absolute)
			if e != nil {
				return bad("pane cwd does not exist")
			}
			info, e := os.Stat(absolute)
			if e != nil || !info.IsDir() {
				return bad("pane cwd must be an existing directory")
			}
			if e = searchable(absolute); e != nil {
				return bad("pane cwd is not searchable")
			}
			w.Panes = append(w.Panes, Pane{Dir: absolute, Command: pane.Command, Split: split})
		}
		result.Windows = append(result.Windows, w)
	}
	return result, nil
}
