package task

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MohamedElashri/txm/pkg/extension"
)

func TestReviewFreezesComparisonAndNeverRunsDiffCommands(t *testing.T) {
	g, repo := repository(t)
	ctx := context.Background()
	base, err := g.text(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	_, common, err := g.repo(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(filepath.Dir(repo), "task")
	if _, _, err = g.run(ctx, repo, "worktree", "add", "-b", "fixture-task", "--", dest, base); err != nil {
		t.Fatal(err)
	}
	ri, _ := extension.Identify(repo)
	ci, _ := extension.Identify(common)
	r := Record{ID: strings.Repeat("a", 32), Repo: repo, Common: common, RepoID: ri, CommonID: ci, Worktree: dest, Branch: "fixture-task", Base: base, Created: true}
	if err = capture(ctx, g, &r); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dest, "odd\tname\nfile"), []byte("hello\x1b[31m\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "--all"}, {"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "work"}, {"config", "diff.external", "/must-not-execute"}} {
		if _, _, err = g.run(ctx, dest, args...); err != nil {
			t.Fatal(err)
		}
	}
	v, err := reviewTask(ctx, g, r, "refs/heads/main", false)
	if err != nil || v.TaskDirty || v.TargetDirty || !v.FastForward || v.CompareFrom != base || v.TaskCommit == base || v.ChangedFiles != 1 || v.Diff != "" || v.TargetWorktree != repo {
		t.Fatalf("review: %+v %v", v, err)
	}
	detailed, err := reviewTask(ctx, g, r, "refs/heads/main", true)
	if err != nil || detailed.Diff == "" || strings.ContainsRune(detailed.Diff, '\x1b') {
		t.Fatal("unsafe diff", detailed, err)
	}
	if err = os.WriteFile(filepath.Join(repo, "untracked"), []byte("dirty"), 0600); err != nil {
		t.Fatal(err)
	}
	dirty, err := reviewTask(ctx, g, r, "refs/heads/main", false)
	if err != nil || !dirty.TargetDirty {
		t.Fatal("target dirty state omitted", err)
	}
	if _, err = reviewTask(ctx, g, r, "HEAD", false); err == nil {
		t.Fatal("nonlocal target accepted")
	}
}
