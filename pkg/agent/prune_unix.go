//go:build linux || darwin

package agent

import (
	"context"
	"encoding/json"
	"os"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

type tombstones struct {
	Schema int      `json:"schema_version"`
	Host   string   `json:"host_id"`
	IDs    []string `json:"ids"`
}

func (s *coordinator) forgotten() (tombstones, error) {
	t := tombstones{1, s.registry.HostID, []string{}}
	data, e := s.state.ReadPrivate("pruned.json", 1<<20)
	if os.IsNotExist(e) {
		return t, nil
	}
	if e != nil {
		return t, e
	}
	if nativeprotocol.Decode(data, &t) != nil || t.Schema != 1 || t.Host != s.registry.HostID || t.IDs == nil || len(t.IDs) > 16384 {
		return t, core.Failure("invalid_config", "invalid pruned-ID history")
	}
	seen := map[string]bool{}
	for _, id := range t.IDs {
		if !runtimepath.ValidID(id) || seen[id] {
			return t, core.Failure("invalid_config", "invalid pruned-ID history")
		}
		seen[id] = true
	}
	return t, nil
}
func (s *coordinator) prune(ctx context.Context, q Request) ([]View, error) {
	r := s.find(q.ID)
	if r == nil {
		return nil, core.Failure("not_found", "agent ID not registered")
	}
	if q.Backend != "" && q.Backend != r.Locator.Backend {
		return nil, core.Failure("invalid_target", "agent backend conflicts with --backend")
	}
	v, e := s.observe(ctx, r)
	if e != nil {
		return []View{v}, e
	}
	if r.Supervised && (v.Lifecycle != "exited" || v.Receipt == nil) {
		return []View{v}, core.Failure("conflict", "prune requires a validated supervised exit receipt")
	}
	history, e := s.forgotten()
	if e != nil {
		return []View{v}, e
	}
	seen := false
	for _, id := range history.IDs {
		if id == q.ID {
			seen = true
		}
	}
	if !seen && len(history.IDs) >= 16384 {
		return []View{v}, core.Failure("conflict", "pruned-ID history limit reached")
	}
	if q.DryRun {
		return []View{v}, nil
	}
	if !seen {
		history.IDs = append(history.IDs, q.ID)
	}
	data, e := json.Marshal(history)
	if e != nil {
		return []View{v}, e
	}
	if e = s.state.WritePrivate("pruned.json", data, true); e != nil {
		return []View{v}, e
	}
	previous := s.registry.Agents
	next := make([]Record, 0, len(previous)-1)
	for _, record := range previous {
		if record.ID != q.ID {
			next = append(next, record)
		}
	}
	s.registry.Agents = next
	if e = saveRegistry(s.state, s.registry); e != nil {
		s.registry.Agents = previous
		return []View{v}, e
	}
	if s.hub != nil {
		delete(s.hub.last, q.ID)
		for sub := range s.hub.subs {
			sub.overflow = true
			sub.reason = "registry_pruned"
			select {
			case sub.resync <- struct{}{}:
			default:
			}
		}
	}
	return []View{v}, nil
}
