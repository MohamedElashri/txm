package dashboard

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"sort"
	"time"
)

type Source interface {
	Sessions(context.Context, string) ([]core.Session, error)
	WatchAgents(context.Context, func([]agent.View)) error
	Preview(context.Context, core.Session, core.CaptureOptions) (core.Snapshot, error)
	Attach(context.Context, core.Session, bool) error
}

// WatchAgentViews consumes the existing M2 API. A resync replaces the full map;
// no names are joined to live M1 sessions and no registry files are inspected.
func WatchAgentViews(ctx context.Context, receive func([]agent.View)) error {
	views := map[string]agent.View{}
	var reconciled time.Time
	return agent.Stream(ctx, "", func(event agent.Event) error {
		if event.Kind == "heartbeat" && time.Since(reconciled) >= 2*time.Second {
			bounded, done := context.WithTimeout(ctx, SourceTimeout)
			reply, e := agent.Call(bounded, agent.Request{Op: "list"}, false)
			done()
			if e != nil {
				return e
			}
			views = map[string]agent.View{}
			for _, v := range reply.Agents {
				views[v.ID] = v
			}
			reconciled = time.Now()
		}
		if event.Kind == "resync_required" {
			reconciled = time.Now()
			views = map[string]agent.View{}
			for _, v := range event.Metadata.Agents {
				views[v.ID] = v
			}
		}
		if event.Metadata.Agent != nil {
			v := *event.Metadata.Agent
			views[v.ID] = v
		}
		next := make([]agent.View, 0, len(views))
		for _, v := range views {
			next = append(next, v)
		}
		sort.Slice(next, func(i, j int) bool { return next[i].ID < next[j].ID })
		receive(next)
		return nil
	})
}
