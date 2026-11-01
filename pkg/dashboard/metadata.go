package dashboard

import (
	"context"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/task"
)

// Called only for explicit agent subscriptions. Plain dashboards never read
// agent/task state or start services for this optional metadata enrichment.
func (m *Model) ApplyAgents(ctx context.Context, views []agent.View, now time.Time) {
	views = views[:min(len(views), MaxSessions)]
	m.Agents = views
	m.AgentUpdated = now
	m.PublicAgents = map[string]agent.PublicView{}
	m.TaskGroups = map[string]string{}
	for _, v := range agent.PublicViews(views) {
		m.PublicAgents[v.ID] = v
	}
	backends := []string{}
	for _, b := range m.Options.Backends {
		if b == "native" || b == "tmux" {
			backends = append(backends, b)
		}
	}
	for start := 0; start < len(views); start += 256 {
		ids := []string{}
		for _, v := range views[start:min(start+256, len(views))] {
			ids = append(ids, v.ID)
		}
		records, e := task.ReadExistingRecords(ctx, core.ReadScope{HostID: m.Options.HostID, Backends: backends, ManagedIDs: ids})
		if e != nil {
			continue
		}
		for _, r := range records {
			v, ok := m.PublicAgents[r.ID]
			if ok && v.InstanceID != nil && *v.InstanceID == r.InstanceID {
				m.TaskGroups[r.ID] = r.Repo + " / " + r.Name + " #" + r.ID[:8]
			}
		}
	}
}
