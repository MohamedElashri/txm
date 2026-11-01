package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

type entry struct {
	Owner   Owner     `json:"owner"`
	Slot    int       `json:"slot"`
	Bytes   int       `json:"bytes"`
	Updated time.Time `json:"updated"`
}
type index struct {
	Schema  int                 `json:"schema_version"`
	Slots   [MaxSamplers]string `json:"slots"`
	Entries []entry             `json:"entries"`
}
type history struct {
	Schema   int      `json:"schema_version"`
	Owner    Owner    `json:"owner"`
	Interval int      `json:"interval_seconds"`
	Samples  []Sample `json:"samples"`
}

func filename(o Owner) string { return "samples-" + o.InstanceID + ".json" }
func readIndex(r *runtimepath.Root) (index, error) {
	v := index{Schema: 1, Entries: []entry{}}
	b, e := r.ReadPrivate("index.json", 1<<20)
	if os.IsNotExist(e) {
		return v, nil
	}
	if e != nil {
		return v, e
	}
	if nativeprotocol.Decode(b, &v) != nil || v.Schema != 1 || v.Entries == nil || len(v.Entries) > maxEntries {
		return v, core.Failure("invalid_config", "invalid metrics index")
	}
	seen := map[string]bool{}
	managed := map[string]bool{}
	total := 0
	for _, entry := range v.Entries {
		if !entry.Owner.valid() || seen[entry.Owner.InstanceID] || managed[entry.Owner.ManagedID] || entry.Slot < 0 || entry.Slot >= MaxSamplers || entry.Bytes < 0 || entry.Bytes > maxHistory || entry.Updated.IsZero() {
			return v, core.Failure("invalid_config", "invalid metrics owner or quota")
		}
		seen[entry.Owner.InstanceID] = true
		managed[entry.Owner.ManagedID] = true
		total += entry.Bytes
	}
	if total > sampleBudget {
		return v, core.Failure("invalid_config", "metrics aggregate quota exceeded")
	}
	for _, id := range v.Slots {
		if id != "" && !runtimepath.ValidID(id) {
			return v, core.Failure("invalid_config", "invalid metrics slot")
		}
	}
	return v, nil
}
func saveIndex(r *runtimepath.Root, v index) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return r.WritePrivate("index.json", b, true)
}
func readHistory(r *runtimepath.Root, e entry) (history, error) {
	v := history{Schema: 1, Owner: e.Owner, Samples: []Sample{}}
	b, err := r.ReadPrivate(filename(e.Owner), MaxPerID)
	if os.IsNotExist(err) && e.Bytes == 0 {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if len(b) > e.Bytes || nativeprotocol.Decode(b, &v) != nil || v.Schema != 1 || v.Owner != e.Owner || v.Samples == nil || len(v.Samples) > maxSamples || v.Interval < 2 || v.Interval > 60 {
		return v, core.Failure("conflict", "metrics history identity or quota mismatch")
	}
	var last time.Time
	for _, s := range v.Samples {
		if s.At.IsZero() || s.At.Before(last) || s.Scope != "child_pid" || (s.State != "available" && s.State != "unavailable") || s.StartIdentity == "" || (s.CPUPercent != nil && (*s.CPUPercent < 0 || *s.CPUPercent > 1000000)) {
			return v, core.Failure("invalid_config", "invalid metrics sample")
		}
		last = s.At
	}
	return v, nil
}
func active(r *runtimepath.Root, v index, e entry) (bool, error) {
	if v.Slots[e.Slot] != e.Owner.InstanceID {
		return false, nil
	}
	f, err := r.Lock(slotName(e.Slot))
	if err == nil {
		_ = f.Close()
		return false, nil
	}
	if isBusy(err) {
		return true, nil
	}
	return false, err
}
func slotName(i int) string { return fmt.Sprintf("sampler-slot-%02d", i) }

// evict removes only inactive histories, oldest first with identity tie-break.
// The index is updated after removal: a crash can over-reserve, never undercount.
func evict(r *runtimepath.Root, v *index, now time.Time, needBytes, needEntries int) error {
	total := 0
	for _, e := range v.Entries {
		total += e.Bytes
	}
	order := append([]entry(nil), v.Entries...)
	sort.Slice(order, func(i, j int) bool {
		if order[i].Updated.Equal(order[j].Updated) {
			return order[i].Owner.InstanceID < order[j].Owner.InstanceID
		}
		return order[i].Updated.Before(order[j].Updated)
	})
	removed := map[string]bool{}
	for _, e := range order {
		if now.Sub(e.Updated) <= MaxAge && total+needBytes <= sampleBudget && len(v.Entries)-len(removed)+needEntries <= maxEntries {
			break
		}
		busy, err := active(r, *v, e)
		if err != nil {
			return err
		}
		if busy {
			continue
		}
		if err = r.RemovePrivate(filename(e.Owner)); err != nil && !os.IsNotExist(err) {
			return err
		}
		removed[e.Owner.InstanceID] = true
		total -= e.Bytes
	}
	kept := v.Entries[:0]
	for _, e := range v.Entries {
		if !removed[e.Owner.InstanceID] {
			kept = append(kept, e)
		}
	}
	v.Entries = kept
	return saveIndex(r, *v)
}

func Read(ctx context.Context, o Owner) (View, error) {
	out := View{Owner: o, Freshness: "unavailable"}
	if !o.valid() {
		return out, core.Failure("invalid_target", "metrics require exact managed and instance IDs")
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	state, e := runtimepath.OpenExistingState()
	if os.IsNotExist(e) {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	defer func() { _ = state.Close() }()
	r, e := state.ChildExisting("metrics")
	if os.IsNotExist(e) {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	defer func() { _ = r.Close() }()
	v, e := readIndex(r)
	if e != nil {
		return out, e
	}
	for _, entry := range v.Entries {
		if entry.Owner != o {
			continue
		}
		h, e := readHistory(r, entry)
		if e != nil {
			if os.IsNotExist(e) {
				return out, nil
			} // publication/retention snapshot raced
			return out, e
		}
		now := time.Now().UTC()
		for _, sample := range h.Samples {
			if now.Sub(sample.At) >= 0 && now.Sub(sample.At) <= MaxAge {
				out.Samples++
				s := sample
				out.Latest = &s
			}
		}
		if out.Latest != nil {
			out.Freshness = "stale"
			if now.Sub(out.Latest.At) <= time.Duration(2*h.Interval)*time.Second {
				out.Freshness = "fresh"
			}
		}
		return out, nil
	}
	return out, nil
}

type sampler struct {
	root     *runtimepath.Root
	lease    *os.File
	owner    Owner
	slot     int
	interval int
}

func reserve(ctx context.Context, o Owner, c Config) (*sampler, error) {
	r, writer, e := extension.Store(ctx, "metrics")
	if e != nil {
		return nil, e
	}
	ok := false
	defer func() {
		_ = writer.Close()
		if !ok {
			_ = r.Close()
		}
	}()
	v, e := readIndex(r)
	if e != nil {
		return nil, e
	}
	for _, prior := range v.Entries {
		if prior.Owner.InstanceID == o.InstanceID || prior.Owner.ManagedID == o.ManagedID {
			return nil, core.Failure("conflict", "metrics instance already recorded; no PID-based recovery")
		}
	}
	var lease *os.File
	slot := -1
	for i := 0; i < MaxSamplers; i++ {
		f, err := r.Lock(slotName(i))
		if err == nil {
			lease = f
			slot = i
			break
		}
		if !isBusy(err) {
			return nil, err
		}
	}
	if slot < 0 {
		return nil, core.Failure("conflict", "32 metrics samplers already active")
	}
	defer func() {
		if !ok {
			_ = lease.Close()
		}
	}()
	v.Slots[slot] = o.InstanceID
	if e = evict(r, &v, time.Now().UTC(), 1024, 1); e != nil {
		return nil, e
	}
	total := 0
	for _, e := range v.Entries {
		total += e.Bytes
	}
	if len(v.Entries) >= maxEntries || total+1024 > sampleBudget {
		return nil, core.Failure("conflict", "metrics storage capacity reached")
	}
	v.Slots[slot] = o.InstanceID
	v.Entries = append(v.Entries, entry{o, slot, 0, time.Now().UTC()})
	if e = saveIndex(r, v); e != nil {
		return nil, e
	}
	ok = true
	return &sampler{r, lease, o, slot, c.IntervalSeconds}, nil
}
func (s *sampler) append(ctx context.Context, sample Sample) error {
	var writer *os.File
	for {
		f, e := s.root.Lock("writer")
		if e == nil {
			writer = f
			break
		}
		if !isBusy(e) {
			return e
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	defer func() { _ = writer.Close() }()
	v, e := readIndex(s.root)
	if e != nil {
		return e
	}
	if e = evict(s.root, &v, sample.At, 0, 0); e != nil {
		return e
	}
	pos := -1
	total := 0
	for i, e := range v.Entries {
		total += e.Bytes
		if e.Owner == s.owner {
			pos = i
		}
	}
	if pos < 0 || v.Slots[s.slot] != s.owner.InstanceID {
		return core.Failure("conflict", "metrics sampler ownership changed")
	}
	h, e := readHistory(s.root, v.Entries[pos])
	if e != nil {
		return e
	}
	h.Interval = s.interval
	kept := []Sample{}
	for _, old := range h.Samples {
		if sample.At.Sub(old.At) >= 0 && sample.At.Sub(old.At) <= MaxAge {
			kept = append(kept, old)
		}
	}
	h.Samples = append(kept, sample)
	if len(h.Samples) > maxSamples {
		h.Samples = h.Samples[len(h.Samples)-maxSamples:]
	}
	b, e := json.Marshal(h)
	if e != nil {
		return e
	}
	need := max(0, len(b)-v.Entries[pos].Bytes)
	if e = evict(s.root, &v, sample.At, need, 0); e != nil {
		return e
	}
	pos = -1
	total = 0
	for i, entry := range v.Entries {
		total += entry.Bytes
		if entry.Owner == s.owner {
			pos = i
		}
	}
	if pos < 0 {
		return core.Failure("conflict", "metrics ownership changed during eviction")
	}
	for {
		b, e = json.Marshal(h)
		if e != nil {
			return e
		}
		if len(b) <= maxHistory && total-v.Entries[pos].Bytes+len(b) <= sampleBudget {
			break
		}
		if len(h.Samples) <= 1 {
			return core.Failure("conflict", "metrics quota reached; other active histories retained")
		}
		h.Samples = h.Samples[1:]
	}
	// Reserve the larger byte count before writing, preserving crash safety.
	if len(b) > v.Entries[pos].Bytes {
		v.Entries[pos].Bytes = len(b)
	}
	v.Entries[pos].Updated = sample.At
	if e = saveIndex(s.root, v); e != nil {
		return e
	}
	if e = s.root.WritePrivate(filename(s.owner), b, true); e != nil {
		return e
	}
	v.Entries[pos].Bytes = len(b)
	return saveIndex(s.root, v)
}

// Start must be called by the supervisor while its child remains unreaped.
// Closing stops all sampling before the supervisor may reap and release the PID.
func Start(ctx context.Context, o Owner, pid int, c Config) (func(), error) {
	if !Supported() {
		return nil, core.Failure("unsupported", "metrics require Linux/macOS and CGO")
	}
	if !o.valid() {
		return nil, core.Failure("invalid_target", "invalid metrics owner")
	}
	if e := c.Validate(); e != nil {
		return nil, e
	}
	first, e := probe(pid)
	if e == nil && first.Parent != os.Getpid() {
		return nil, core.Failure("denied", "metrics require the supervisor own unreaped child")
	}
	if e != nil {
		return nil, e
	}
	s, e := reserve(ctx, o, c)
	if e != nil {
		return nil, e
	}
	bounded, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = s.lease.Close(); _ = s.root.Close() }()
		previous := counter{}
		current := first
		for {
			if e = s.append(bounded, project(previous, current)); e != nil {
				return
			}
			previous = current
			timer := time.NewTimer(time.Duration(c.IntervalSeconds) * time.Second)
			select {
			case <-bounded.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			current, e = probe(pid)
			if e != nil || current.Identity != first.Identity {
				_ = s.append(bounded, Sample{At: time.Now().UTC(), Scope: "child_pid", Source: "kernel child process counters", State: "unavailable", StartIdentity: first.Identity})
				return
			}
		}
	}()
	return func() { cancel(); <-done }, nil
}

// errors.Is preserves the platform lock error through runtimepath's wrapping.
func isBusy(e error) bool { return errors.Is(e, errWouldBlock()) }
