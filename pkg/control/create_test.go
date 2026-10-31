//go:build linux || darwin

package control

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateReservationSurvivesUncertainFailureAndPolicyChange(t *testing.T) {
	root := t.TempDir()
	_ = os.Chmod(root, 0700)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("HOME", root)
	exe := filepath.Join(root, "worker")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	p := CreatePolicy{1, "11111111111111111111111111111111", []CreateTemplate{{ID: "task", Kind: "task", Name: "fixture", Backend: "native", Directory: root, BaseCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Argv: []string{exe}}}}
	c, err := NewCreator(p)
	if err != nil {
		t.Fatal(err)
	}
	in := CreateInput{"22222222222222222222222222222222", "task"}
	first, err := c.Create(context.Background(), "task", in)
	if err != nil || first.State != "uncertain" || first.ResourceID == "" {
		t.Fatalf("expected retained failed intent: %+v %v", first, err)
	}
	// Not a Git repository: creation fails after reservation. Restart and retry
	// must return the same identity without creating another launch intent.
	c, err = NewCreator(p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := c.Create(context.Background(), "task", in)
	if err != nil || again != first {
		t.Fatalf("replayed uncertain operation: %+v %v", again, err)
	}
	p.Templates[0].Name = "changed"
	c, err = NewCreator(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Create(context.Background(), "task", in); err == nil {
		t.Fatal("changed authorization reused namespace")
	}
}

func TestJournalCapacityPreservesPermanentTombstones(t *testing.T) {
	root := t.TempDir()
	_ = os.Chmod(root, 0700)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("HOME", root)
	exe := filepath.Join(root, "worker")
	if err := os.WriteFile(exe, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	c, err := NewCreator(CreatePolicy{1, "11111111111111111111111111111111", []CreateTemplate{{ID: "worker", Kind: "agent", Name: "fixture", Backend: "native", Directory: root, Argv: []string{exe}}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	host, err := runtimepath.HostID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	r, l, err := extension.Store(ctx, "mcp-"+c.policy.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	j := createJournal{1, c.digest, host, []Creation{}}
	hash := digestJSON(struct{ Policy, Kind, Template string }{c.digest, "agent", "worker"})
	for i := 1; i <= 1024; i++ {
		j.Entries = append(j.Entries, Creation{fmt.Sprintf("%032x", i), fmt.Sprintf("%032x", i+1024), "agent", "uncertain", hash})
	}
	b, _ := json.Marshal(j)
	if err = r.WritePrivate("operations.json", b, true); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	_ = r.Close()
	if _, err = c.Create(ctx, "agent", CreateInput{fmt.Sprintf("%032x", 4000), "worker"}); err == nil {
		t.Fatal("capacity allowed mutation")
	}
	prior, err := c.Create(ctx, "agent", CreateInput{fmt.Sprintf("%032x", 1), "worker"})
	if err != nil || prior != j.Entries[0] {
		t.Fatal("permanent reservation lost", prior, err)
	}
}
