package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCPUUnitsAndIdentity(t *testing.T) {
	now := time.Now()
	a := counter{Identity: "start-A", CPU: time.Second, RSS: 123, At: now}
	b := counter{Identity: "start-A", CPU: 3 * time.Second, RSS: 456, At: now.Add(2 * time.Second)}
	s := project(a, b)
	if s.CPUPercent == nil || *s.CPUPercent != 100 || s.RSSBytes == nil || *s.RSSBytes != 456 || s.Scope != "child_pid" {
		t.Fatal(s)
	}
	b.Identity = "reused-PID"
	if project(a, b).CPUPercent != nil {
		t.Fatal("PID reuse borrowed old CPU baseline")
	}
	if (Config{1}).Validate() == nil || (Config{61}).Validate() == nil {
		t.Fatal("unbounded interval")
	}
}
func fixtureOwner(i int) Owner {
	return Owner{fmt.Sprintf("%032x", i), fmt.Sprintf("%032x", i+2048), "native"}
}

func metricsTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNonChildPIDCannotStartSampler(t *testing.T) {
	if !Supported() {
		t.Skip("native metrics platform")
	}
	root := metricsTempDir(t)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	if stop, e := Start(context.Background(), fixtureOwner(1), os.Getpid(), Config{2}); e == nil {
		stop()
		t.Fatal("non-child PID accepted")
	}
	if _, e := os.Stat(filepath.Join(root, "state")); !os.IsNotExist(e) {
		t.Fatal("rejected sampler created state", e)
	}
}

func TestInactiveEvictionOrderAndAggregateQuotaValidation(t *testing.T) {
	if !Supported() {
		t.Skip("native metrics platform")
	}
	root := metricsTempDir(t)
	_ = os.Chmod(root, 0700)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("HOME", root)
	s, e := reserve(context.Background(), fixtureOwner(1), Config{2})
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.lease.Close(); _ = s.root.Close() }()
	idx, e := readIndex(s.root)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	for i := maxEntries; i >= 2; i-- {
		idx.Entries = append(idx.Entries, entry{Owner: fixtureOwner(i), Slot: 1, Updated: now.Add(-time.Hour)})
	}
	if e = evict(s.root, &idx, now, 0, 1); e != nil {
		t.Fatal(e)
	}
	if len(idx.Entries) != maxEntries-1 {
		t.Fatal("entry quota not reserved")
	}
	for _, entry := range idx.Entries {
		if entry.Owner == fixtureOwner(2) {
			t.Fatal("oldest identity tie-break not evicted")
		}
	}
	if idx.Entries[0].Owner != s.owner {
		t.Fatal("active owner evicted")
	}
	// A corrupt or over-reserved byte index must fail before reading samples.
	for i := 0; i < 61; i++ {
		idx.Entries[i].Bytes = maxHistory
	}
	if e = saveIndex(s.root, idx); e != nil {
		t.Fatal(e)
	}
	if _, e = readIndex(s.root); e == nil {
		t.Fatal("aggregate quota violation accepted")
	}
}
func TestSamplerCapacityAndRetention(t *testing.T) {
	if !Supported() {
		t.Skip("native metrics platform")
	}
	root := metricsTempDir(t)
	_ = os.Chmod(root, 0700)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("HOME", root)
	ctx := context.Background()
	samplers := []*sampler{}
	defer func() {
		for _, s := range samplers {
			_ = s.lease.Close()
			_ = s.root.Close()
		}
	}()
	for i := 1; i <= MaxSamplers; i++ {
		s, e := reserve(ctx, fixtureOwner(i), Config{2})
		if e != nil {
			t.Fatal(e)
		}
		samplers = append(samplers, s)
	}
	if _, e := reserve(ctx, fixtureOwner(100), Config{2}); e == nil {
		t.Fatal("33rd sampler was admitted")
	}
	s := samplers[0]
	at := time.Now().UTC()
	sample := Sample{At: at, Scope: "child_pid", Source: "fixture", State: "available", StartIdentity: "pid-start"}
	if e := s.append(ctx, sample); e != nil {
		t.Fatal(e)
	}
	v, e := Read(ctx, s.owner)
	if e != nil || v.Samples != 1 || v.Latest.StartIdentity != "pid-start" {
		t.Fatal(v, e)
	}
	// Reserve bytes in index before publishing a large history, just as the
	// production writer does. One append must enforce both age and count limits.
	idx, e := readIndex(s.root)
	if e != nil {
		t.Fatal(e)
	}
	h := history{1, s.owner, 2, []Sample{}}
	for i := 0; i < maxSamples; i++ {
		old := sample
		old.At = at.Add(-25 * time.Hour)
		h.Samples = append(h.Samples, old)
	}
	b, _ := json.Marshal(h)
	idx.Entries[0].Bytes = len(b)
	if e = saveIndex(s.root, idx); e != nil {
		t.Fatal(e)
	}
	if e = s.root.WritePrivate(filename(s.owner), b, true); e != nil {
		t.Fatal(e)
	}
	if e = s.append(ctx, sample); e != nil {
		t.Fatal(e)
	}
	v, e = Read(ctx, s.owner)
	if e != nil || v.Samples != 1 {
		t.Fatal("age eviction failed", v, e)
	}
	_ = samplers[1].lease.Close()
	_ = samplers[1].root.Close()
	replacement, e := reserve(ctx, fixtureOwner(101), Config{2})
	if e != nil {
		t.Fatal("released slot not reclaimed", e)
	}
	samplers = append(samplers, replacement)
}
