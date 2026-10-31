//go:build linux || darwin

package agent

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"time"
)

func (s *coordinator) terminate(ctx context.Context, id string) ([]View, error) {
	for i := range s.registry.Agents {
		r := &s.registry.Agents[i]
		if r.ID != id {
			continue
		}
		if !r.Supervised {
			return nil, core.Failure("conflict", "only supervised registrations can be terminated")
		}
		v, e := s.observe(ctx, r)
		if e != nil {
			return []View{v}, e
		}
		if v.Receipt != nil {
			return []View{v}, nil
		}
		if v.Lifecycle != "running" {
			return []View{v}, core.Failure("conflict", "supervised process state is unknown")
		}
		b, e := backend.NewBackend(r.Locator.Backend, ctx)
		if e != nil {
			return []View{v}, e
		}
		if e = backend.TerminateManaged(ctx, b, r.Binding); e != nil {
			return []View{v}, e
		}
		for {
			v, e = s.observe(ctx, r)
			if e != nil || v.Receipt != nil {
				return []View{v}, e
			}
			select {
			case <-ctx.Done():
				return []View{v}, ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	return nil, core.Failure("not_found", "agent ID not registered")
}
