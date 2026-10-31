package task

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
)

// Git prints some origin paths relative to the invocation directory. Normalize
// only origin fields, retaining every key/value byte in the private digest.
func (g git) configuration(ctx context.Context, dir string) ([]byte, int, error) {
	b, code, e := g.run(ctx, dir, "config", "--null", "--show-origin", "--list")
	if e != nil {
		return nil, code, e
	}
	fields := bytes.Split(b, []byte{0})
	for i := 0; i+1 < len(fields); i += 2 {
		origin := string(fields[i])
		if strings.HasPrefix(origin, "file:") {
			path := strings.TrimPrefix(origin, "file:")
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			fields[i] = []byte("file:" + filepath.Clean(path))
		}
	}
	return bytes.Join(fields, []byte{0}), code, nil
}

type Review struct {
	ID             string             `json:"id"`
	BaseCommit     string             `json:"base_commit"`
	TaskCommit     string             `json:"task_commit"`
	TargetRef      string             `json:"target_ref"`
	TargetCommit   string             `json:"target_commit"`
	CompareFrom    string             `json:"compare_from"`
	CompareTo      string             `json:"compare_to"`
	TaskDirty      bool               `json:"task_dirty"`
	TargetDirty    bool               `json:"target_dirty"`
	TargetWorktree string             `json:"target_worktree,omitempty"`
	TargetIdentity extension.Identity `json:"target_identity"`
	ChangedFiles   int                `json:"changed_files"`
	FastForward    bool               `json:"fast_forward"`
	ConfigHash     string             `json:"config_hash"`
	GitHash        string             `json:"git_executable_hash"`
	Diff           string             `json:"diff,omitempty"`
	DiffTruncated  bool               `json:"diff_truncated"`
	MergeMode      string             `json:"merge_mode"`
	Handoff        string             `json:"handoff"`
}

func taskRecordExisting(ctx context.Context, id string) (Record, error) {
	rows, err := ReadExistingRecords(ctx, core.ReadScope{Backends: []string{"native", "tmux"}, ManagedIDs: []string{id}})
	if err != nil {
		return Record{}, err
	}
	if len(rows) != 1 {
		return Record{}, core.Failure("not_found", "owned task is unavailable")
	}
	return rows[0], nil
}

func ReviewTask(ctx context.Context, id, target string, diff bool) (Review, error) {
	r, err := taskRecordExisting(ctx, id)
	if err != nil {
		return Review{}, err
	}
	g, err := newGit(false)
	if err != nil {
		return Review{}, err
	}
	return reviewTask(ctx, g, r, target, diff)
}
func reviewTask(ctx context.Context, g git, r Record, target string, diff bool) (Review, error) {
	var v Review
	gitHash, hashErr := extension.ExecutableHash(g.exe)
	if hashErr != nil {
		return v, hashErr
	}
	if !r.Created || r.Cleaned {
		return v, core.Failure("conflict", "review requires an existing owned task worktree")
	}
	if err := verify(ctx, g, r); err != nil {
		return v, err
	}
	if !strings.HasPrefix(target, "refs/heads/") || len(target) > 240 {
		return v, core.Failure("invalid_target", "review target must be an exact local refs/heads ref")
	}
	if _, _, err := g.run(ctx, r.Repo, "check-ref-format", target); err != nil {
		return v, core.Failure("invalid_target", "invalid review target ref")
	}
	if _, code, _ := g.run(ctx, r.Repo, "symbolic-ref", "--quiet", target); code != 1 {
		return v, core.Failure("invalid_target", "symbolic target is unsupported")
	}
	head, err := g.text(ctx, r.Worktree, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil || !commitPattern().MatchString(head) {
		return v, core.Failure("conflict", "task head is unavailable")
	}
	targetOID, err := g.text(ctx, r.Repo, "rev-parse", "--verify", "--end-of-options", target+"^{commit}")
	if err != nil || !commitPattern().MatchString(targetOID) {
		return v, core.Failure("invalid_target", "target must resolve to a local committed branch")
	}
	v = Review{ID: r.ID, BaseCommit: r.Base, TaskCommit: head, TargetRef: target, TargetCommit: targetOID, CompareFrom: r.Base, CompareTo: head, MergeMode: "manual_ref_only", Handoff: "Review exact commits and worktree state; ref-only apply is disabled because checkout creation is not serialized."}
	status := func(path string) (bool, error) {
		b, _, e := g.run(ctx, path, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching", "--ignore-submodules=all")
		return len(b) > 0, e
	}
	v.TaskDirty, err = status(r.Worktree)
	if err != nil {
		return v, err
	}
	_, code, err := g.run(ctx, r.Repo, "merge-base", "--is-ancestor", targetOID, head)
	if code != 0 && code != 1 {
		return v, err
	}
	v.FastForward = code == 0
	config, _, err := g.configuration(ctx, r.Repo)
	if err != nil {
		return v, err
	}
	workConfig, _, err := g.configuration(ctx, r.Worktree)
	if err != nil {
		return v, err
	}
	config = append(config, workConfig...)
	listing, _, err := g.run(ctx, r.Repo, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return v, err
	}
	for _, block := range bytes.Split(listing, []byte{0, 0}) {
		var path, branch string
		unsafe := false
		for _, field := range bytes.Split(block, []byte{0}) {
			s := string(field)
			switch {
			case strings.HasPrefix(s, "worktree "):
				path = strings.TrimPrefix(s, "worktree ")
			case strings.HasPrefix(s, "branch "):
				branch = strings.TrimPrefix(s, "branch ")
			case strings.HasPrefix(s, "locked"), strings.HasPrefix(s, "prunable"):
				unsafe = true
			}
		}
		if branch != target {
			continue
		}
		if unsafe || v.TargetWorktree != "" || !filepath.IsAbs(path) {
			return v, core.Failure("conflict", "target worktree routing is ambiguous, locked or prunable")
		}
		identity, e := extension.Identify(path)
		if e != nil {
			return v, core.Failure("conflict", "target worktree is inaccessible or unsafe")
		}
		root, common, e := g.repo(ctx, path)
		if e != nil || root != path || common != r.Common {
			return v, core.Failure("conflict", "target worktree repository changed")
		}
		if e = g.filters(ctx, path); e != nil {
			return v, e
		}
		v.TargetWorktree = path
		v.TargetIdentity = identity
		v.TargetDirty, e = status(path)
		if e != nil {
			return v, e
		}
		c, _, e := g.configuration(ctx, path)
		if e != nil {
			return v, e
		}
		config = append(config, c...)
		v.MergeMode = "checked_out_requires_quiescence"
		v.Handoff = "Apply requires explicit operator quiescence, exact task/target OIDs and review digest. No automatic push or cleanup."
	}
	sum := sha256.Sum256(config)
	v.ConfigHash = hex.EncodeToString(sum[:])
	v.GitHash = gitHash
	files, _, err := g.run(ctx, r.Worktree, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", r.Base, head, "--")
	if err != nil {
		return v, err
	}
	v.ChangedFiles = bytes.Count(files, []byte{0})
	if diff {
		b, _, e := g.run(ctx, r.Worktree, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--no-color", r.Base, head, "--")
		if e != nil {
			return v, e
		}
		v.DiffTruncated = len(b) > 16<<10
		if v.DiffTruncated {
			b = b[:16<<10]
		}
		v.Diff = core.Plain(b)
	}
	return v, nil
}
