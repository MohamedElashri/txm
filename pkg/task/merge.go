package task

import (
	"bytes"
	"context"
	"encoding/json"
	"os"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
)

type MergeOptions struct {
	ID, Target, ExpectedTask, ExpectedTarget, ReviewDigest, Confirm string
	DryRun, Quiescent                                               bool
}
type MergeResult struct {
	Schema       int    `json:"schema_version"`
	State        string `json:"state"`
	Review       Review `json:"review"`
	ReviewDigest string `json:"review_digest"`
}

func reviewDigest(r Review) string {
	r.Diff = ""
	r.DiffTruncated = false
	b, _ := json.Marshal(r)
	return hashBytes(b)
}

// MergeTask supports only a checked-out, owned clean target. The operator's
// explicit quiescence assertion is the concurrency boundary: all other writers
// must be stopped, including escaped descendants and external Git clients.
// Ref-only updates remain disabled; no lock here serializes checkout creation.
func MergeTask(ctx context.Context, o MergeOptions) (MergeResult, error) {
	out := MergeResult{Schema: 1, State: "failed_before_mutation"}
	if o.DryRun {
		r, e := ReviewTask(ctx, o.ID, o.Target, false)
		out.Review = r
		out.ReviewDigest = reviewDigest(r)
		out.State = "dry_run"
		return out, e
	}
	if !o.Quiescent || o.Confirm != o.ID || !commitPattern().MatchString(o.ExpectedTask) || !commitPattern().MatchString(o.ExpectedTarget) || len(o.ReviewDigest) != 64 {
		return out, core.Failure("denied", "merge requires operator quiescence, full task confirmation, reviewed OIDs and digest")
	}
	s, e := open(ctx)
	if e != nil {
		return out, e
	}
	defer s.close()
	r, e := s.find(o.ID)
	if e != nil {
		return out, e
	}
	name := "merge-" + r.ID + ".json"
	prior, e := s.root.ReadPrivate(name, 64<<10)
	if e == nil {
		var old MergeResult
		if nativeprotocol.Decode(prior, &old) != nil || old.Schema != 1 || old.Review.ID != r.ID || (old.State != "uncertain" && old.State != "applied") {
			return out, core.Failure("conflict", "invalid merge journal; inspect manually")
		}
		if old.ReviewDigest != o.ReviewDigest || old.Review.TaskCommit != o.ExpectedTask || old.Review.TargetCommit != o.ExpectedTarget || old.Review.TargetRef != o.Target {
			return out, core.Failure("conflict", "task already has another merge intent; no automatic retry")
		}
		// Return the recorded result without reapplying. Applied is a historical
		// receipt, not a claim that an external actor has preserved the branch.
		if old.State == "uncertain" {
			return old, core.Failure("conflict", "merge intent remains uncertain; inspect refs and worktrees manually; no retry attempted")
		}
		return old, nil
	}
	if !os.IsNotExist(e) {
		return out, e
	}
	g, e := newGit(false)
	if e != nil {
		return out, e
	}
	v, e := reviewTask(ctx, g, *r, o.Target, false)
	out.Review = v
	out.ReviewDigest = reviewDigest(v)
	if e != nil {
		return out, e
	}
	if v.TargetWorktree == "" {
		return out, core.Failure("unsupported", "ref-only merge is disabled: concurrent checkout creation is not serialized; use an explicitly reviewed manual Git handoff")
	}
	if v.TaskCommit != o.ExpectedTask || v.TargetCommit != o.ExpectedTarget || out.ReviewDigest != o.ReviewDigest {
		return out, core.Failure("conflict", "reviewed merge preconditions changed")
	}
	if v.TaskDirty || v.TargetDirty || !v.FastForward || v.TargetWorktree == r.Worktree {
		return out, core.Failure("conflict", "merge requires distinct clean worktrees and fast-forward ancestry")
	}
	if r.InstanceID == "" {
		return out, core.Failure("conflict", "merge requires a pinned main process exit receipt")
	}
	receipt, e := ReadExistingReceipt(ctx, *r)
	if e != nil || receipt.ExitCode == nil || *receipt.ExitCode != 0 {
		return out, core.Failure("conflict", "merge requires successful main process exit proof")
	}
	setup, e := readSetup(s.root, r.ID)
	if e != nil {
		return out, e
	}
	if setup != nil && (setup.State != "succeeded" || !setup.MainLaunch) {
		return out, core.Failure("conflict", "merge requires successful setup and main launch")
	}
	if e = guardSetupExit(ctx, s.root, *r); e != nil {
		return out, e
	}
	if e = agent.AssertManagedQuiescence(ctx); e != nil {
		return out, e
	}
	// Submodule worktree state and sparse indexes require a separate contract.
	for _, dir := range []string{r.Worktree, v.TargetWorktree} {
		index, _, err := g.run(ctx, dir, "ls-files", "--stage", "-z")
		if err != nil {
			return out, err
		}
		for _, entry := range bytes.Split(index, []byte{0}) {
			if bytes.HasPrefix(entry, []byte("160000 ")) {
				return out, core.Failure("unsupported", "merge with submodules requires manual review")
			}
		}
		for _, key := range []string{"core.sparseCheckout", "index.sparse"} {
			value, code, err := g.run(ctx, dir, "config", "--bool", "--get", key)
			if code != 0 && code != 1 {
				return out, err
			}
			if string(bytes.TrimSpace(value)) == "true" {
				return out, core.Failure("unsupported", "sparse worktree merge requires manual review")
			}
		}
	}
	// An immutable intent precedes Git. Any interruption from this point is
	// uncertain, including a crash after Git succeeded but before final save.
	out.State = "uncertain"
	save := func(replace bool) error { b, _ := json.Marshal(out); return s.root.WritePrivate(name, b, replace) }
	if e = save(false); e != nil {
		return out, e
	}
	again, e := reviewTask(ctx, g, *r, o.Target, false)
	if e != nil || reviewDigest(again) != out.ReviewDigest {
		return out, core.Failure("conflict", "merge preconditions changed after intent; retained uncertain journal requires inspection")
	}
	if _, _, e = g.run(ctx, v.TargetWorktree, "-c", "merge.autoStash=false", "merge", "--ff-only", "--no-edit", "--no-stat", v.TaskCommit); e != nil {
		return out, core.Failure("conflict", "Git merge did not complete cleanly; inspect retained uncertain intent, do not retry or reset automatically")
	}
	post, e := reviewTask(ctx, g, *r, o.Target, false)
	if e != nil || post.TargetCommit != v.TaskCommit || post.TaskCommit != v.TaskCommit || post.TargetDirty || post.TaskDirty || post.TargetIdentity != v.TargetIdentity || post.TargetWorktree != v.TargetWorktree || post.ConfigHash != v.ConfigHash || post.GitHash != v.GitHash {
		return out, core.Failure("conflict", "merge postconditions uncertain; inspect refs/index/worktree, no rollback attempted")
	}
	out.State = "applied"
	if e = save(true); e != nil {
		out.State = "uncertain"
		return out, core.Failure("conflict", "merge completed but durable outcome is uncertain; inspect before further action")
	}
	return out, nil
}
