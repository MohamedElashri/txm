//go:build linux || darwin

package agent

import (
	"context"
	"net"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/localipc"
)

func (s *coordinator) snapshot(ctx context.Context, id string) ([]View, error) {
	result := []View{}
	for i := range s.registry.Agents {
		if id != "" && s.registry.Agents[i].ID != id {
			continue
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		v, e := s.observe(ctx, &s.registry.Agents[i])
		if e != nil {
			return nil, e
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	if id != "" && len(result) == 0 {
		return nil, core.Failure("not_found", "agent ID not registered")
	}
	return result, nil
}
func (s *coordinator) stream(ctx context.Context, c net.Conn, q Request) {
	var sub *subscription
	fail := func(e error) {
		_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_ = localipc.Write(c, Reply{SchemaVersion: 1, Epoch: s.epoch, Agents: []View{}, Error: core.Normalize(e)})
	}
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return
	}
	if e := q.validate(); e != nil {
		<-s.gate
		fail(e)
		return
	}
	if len(s.hub.subs) >= 8 {
		<-s.gate
		fail(core.Failure("conflict", "event subscriber limit reached"))
		return
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	views, e := s.snapshot(bounded, q.ID)
	cancel()
	if e != nil {
		<-s.gate
		fail(e)
		return
	}
	sub = s.hub.add(q.ID)
	reason := "connected"
	if q.LastEpoch != "" {
		reason = "resubscribed"
		if q.LastEpoch != s.epoch {
			reason = "epoch_changed"
		}
	}
	initial := s.hub.event("resync_required", "coordinator", nil, EventMetadata{Reason: reason, Agents: views})
	<-s.gate
	select {
	case s.wake <- struct{}{}:
	default:
	}
	defer func() {
		s.gate <- struct{}{}
		delete(s.hub.subs, sub)
		<-s.gate
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}()
	write := func(e Event) error {
		_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
		return localipc.Write(c, Reply{SchemaVersion: 1, Epoch: s.epoch, Agents: []View{}, Event: &e})
	}
	if write(initial) != nil {
		return
	}
	// Reading one byte detects a disconnected or protocol-violating subscriber.
	disconnected := make(chan struct{})
	go func() { var byte [1]byte; _, _ = c.Read(byte[:]); close(disconnected) }()
	defer func() { _ = c.Close(); <-disconnected }()
	for {
		select {
		case <-ctx.Done():
			return
		case <-disconnected:
			return
		case event := <-sub.queue:
			if write(event) != nil {
				return
			}
		case <-sub.resync:
			select {
			case s.gate <- struct{}{}:
			case <-ctx.Done():
				return
			}
			bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
			views, e := s.snapshot(bounded, sub.id)
			cancel()
			if e != nil {
				<-s.gate
				fail(e)
				return
			}
			for {
				select {
				case <-sub.queue:
					continue
				default:
					goto drained
				}
			}
		drained:
			sub.overflow = false
			reason := sub.reason
			if reason == "" {
				reason = "overflow"
			}
			sub.reason = ""
			event := s.hub.event("resync_required", "coordinator", nil, EventMetadata{Reason: reason, Agents: views})
			<-s.gate
			if write(event) != nil {
				return
			}
		}
	}
}
func (s *coordinator) watch(ctx context.Context) {
	var timer *time.Timer
	var ticks <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	lastHeartbeat := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticks:
		}
		select {
		case s.gate <- struct{}{}:
		case <-ctx.Done():
			return
		}
		if len(s.hub.subs) == 0 {
			if timer != nil {
				timer.Stop()
			}
			ticks = nil
			<-s.gate
			continue
		}
		ids := map[string]bool{}
		all := false
		for sub := range s.hub.subs {
			if sub.id == "" {
				all = true
			}
			ids[sub.id] = true
		}
		bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
		for i := range s.registry.Agents {
			if bounded.Err() != nil {
				break
			}
			r := &s.registry.Agents[i]
			if all || ids[r.ID] {
				_, _ = s.observe(bounded, r)
			}
		}
		cancel()
		if time.Since(lastHeartbeat) >= time.Second {
			s.hub.emit("heartbeat", "coordinator", nil)
			lastHeartbeat = time.Now()
		}
		<-s.gate
		if timer == nil {
			timer = time.NewTimer(500 * time.Millisecond)
		} else {
			timer.Reset(500 * time.Millisecond)
		}
		ticks = timer.C
	}
}
