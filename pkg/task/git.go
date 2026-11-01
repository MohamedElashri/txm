// Package task coordinates opt-in Git worktrees with existing managed agents.
package task

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type git struct {
	exe   string
	allow bool
}

func newGit(allow bool) (git, error) {
	p, e := extension.Resolve("git")
	if e != nil {
		return git{}, e
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		return git{}, e
	}
	return git{p, allow}, nil
}
func (g git) run(ctx context.Context, dir string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	env := []string{}
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GIT_") {
			env = append(env, v)
		}
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	flags := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "submodule.recurse=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	if !g.allow {
		flags = append(flags, "-c", "core.hooksPath=/dev/null")
	}
	return extension.Output(ctx, g.exe, dir, env, append(flags, args...)...)
}
func (g git) text(ctx context.Context, dir string, args ...string) (string, error) {
	b, _, e := g.run(ctx, dir, args...)
	return strings.TrimSuffix(string(b), "\n"), e
}
func (g git) filters(ctx context.Context, dir string) error {
	if g.allow {
		return nil
	}
	b, code, e := g.run(ctx, dir, "config", "--name-only", "--get-regexp", `^filter\..*\.(clean|smudge|process|required)$`)
	if code == 1 {
		return nil
	}
	if e != nil {
		return e
	}
	if len(b) > 0 {
		return core.Failure("denied", "repository has external Git filters; review and explicitly use --allow-repo-commands")
	}
	return nil
}

var commitPattern = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^([a-f0-9]{40}|[a-f0-9]{64})$`) })

func (g git) repo(ctx context.Context, p string) (string, string, error) {
	a, e := filepath.Abs(p)
	if e != nil {
		return "", "", e
	}
	if e = extension.NoSymlinks(a); e != nil {
		return "", "", e
	}
	root, e := g.text(ctx, a, "rev-parse", "--show-toplevel")
	if e != nil {
		return "", "", core.Failure("invalid_target", "task requires a Git working repository")
	}
	if e = extension.NoSymlinks(root); e != nil {
		return "", "", e
	}
	common, e := g.text(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return "", "", e
	}
	if e = extension.NoSymlinks(common); e != nil {
		return "", "", e
	}
	return root, common, nil
}
