package task

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/extension"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repository(t *testing.T) (git, string) {
	t.Helper()
	if os.PathSeparator != '/' {
		t.Skip("Unix ownership")
	}
	exe, e := exec.LookPath("git")
	if e != nil {
		t.Skip("git unavailable")
	}
	home, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	if e = os.Chmod(home, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	root := filepath.Join(home, "repo")
	if e = os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	// Production strips GIT_* routing overrides. Isolate ambient system/global
	// config in a test-only executable wrapper after that sanitization boundary.
	wrapper := filepath.Join(home, "fixture-git")
	quoted := "'" + strings.ReplaceAll(exe, "'", "'\\''") + "'"
	script := "#!/bin/sh\nexport GIT_CONFIG_NOSYSTEM=1\nexport GIT_CONFIG_GLOBAL=/dev/null\nexec " + quoted + " \"$@\"\n"
	if e = os.WriteFile(wrapper, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	g := git{exe: wrapper}
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "fixture"}} {
		if _, _, e = g.run(context.Background(), root, args...); e != nil {
			t.Fatal(e)
		}
	}
	return g, root
}
func TestGitRoutingFiltersAndFreshBranches(t *testing.T) {
	g, repo := repository(t)
	ctx := context.Background()
	t.Setenv("GIT_DIR", "/unrelated/must-not-use")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", "/unrelated")
	root, _, e := g.repo(ctx, repo)
	if e != nil || root != repo {
		t.Fatal(root, e)
	}
	if e = checkBranch(ctx, g, repo, "main"); e == nil {
		t.Fatal("existing branch accepted")
	}
	for _, branch := range []string{"../other", "--force", "bad name"} {
		if e = checkBranch(ctx, g, repo, branch); e == nil {
			t.Fatal(branch)
		}
	}
	if e = checkBranch(ctx, g, repo, "fresh"); e != nil {
		t.Fatal(e)
	}
	if _, _, e = g.run(ctx, repo, "config", "filter.fixture.smudge", "touch should-never-run"); e != nil {
		t.Fatal(e)
	}
	if e = g.filters(ctx, repo); e == nil {
		t.Fatal("external filter accepted")
	}
	g.allow = true
	if e = g.filters(ctx, repo); e != nil {
		t.Fatal(e)
	}
}
func TestWorktreeIdentityAndHooks(t *testing.T) {
	g, repo := repository(t)
	// Simulate runner-level Git LFS/filter config without editing host config.
	if e := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), []byte("[filter \"ambient-fixture\"]\nsmudge = /bin/false\n"), 0600); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	_, common, e := g.repo(ctx, repo)
	if e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(repo, "hook-executed")
	hook := filepath.Join(common, "hooks", "post-checkout")
	if e = os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(filepath.Dir(repo), "worktree")
	if _, _, e = g.run(ctx, repo, "worktree", "add", "-b", "task-fixture", "--", dest, "HEAD"); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("hook executed")
	}
	ri, _ := extension.Identify(repo)
	ci, _ := extension.Identify(common)
	r := Record{Repo: repo, Common: common, RepoID: ri, CommonID: ci, Worktree: dest, Branch: "task-fixture"}
	if e = capture(ctx, g, &r); e != nil {
		t.Fatal(e)
	}
	if e = verify(ctx, g, r); e != nil {
		t.Fatal(e)
	}
	old := filepath.Join(filepath.Dir(repo), "old-worktree")
	if e = os.Rename(dest, old); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(old, dest); e != nil {
		t.Fatal(e)
	}
	if verify(ctx, g, r) == nil {
		t.Fatal("symlink replacement accepted")
	}
}
