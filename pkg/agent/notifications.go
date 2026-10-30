package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"
)

// Notifications is explicitly opt-in. Resync snapshots never execute actions.
// Slow or failing actions cannot hold up coordinator fanout; the local action
// queue is bounded and notifications are best effort.
func Notifications(ctx context.Context, run func(context.Context, []byte) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	queue := make(chan []byte, 8)
	var worker sync.WaitGroup
	worker.Add(1)
	defer worker.Wait()
	// Cancel before waiting, including when Stream returns a protocol error.
	defer cancel()
	go func() {
		defer worker.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case payload := <-queue:
				action, done := context.WithTimeout(ctx, 2*time.Second)
				_ = run(action, payload)
				done()
			}
		}
	}()
	last := map[string]time.Time{}
	seen := map[string]string{}
	return Stream(ctx, "", func(e Event) error {
		if e.Kind == "resync_required" {
			for _, v := range e.Metadata.Agents {
				if v.Receipt != nil {
					seen[v.ID+":exited"] = "exit:" + v.Receipt.FinishedAt.String()
				}
				if v.TurnCompleteAt != nil {
					seen[v.ID+":turn_complete"] = "turn:" + v.TurnCompleteAt.String()
				}
				if v.Attention == "needs_input" && v.Hooks != nil {
					seen[v.ID+":needs_input"] = "input:" + strconv.FormatUint(v.Hooks.Sequence, 10)
				}
			}
			return nil
		}
		if e.Metadata.Agent == nil {
			return nil
		}
		v := e.Metadata.Agent
		kind, key := "", ""
		switch {
		case v.Receipt != nil:
			kind = "exited"
			key = "exit:" + v.Receipt.FinishedAt.String()
		case v.TurnCompleteAt != nil && e.Kind == "turn_complete":
			kind = "turn_complete"
			key = "turn:" + v.TurnCompleteAt.String()
		case v.Attention == "needs_input" && v.Hooks != nil:
			kind = "needs_input"
			key = "input:" + strconv.FormatUint(v.Hooks.Sequence, 10)
		default:
			return nil
		}
		if seen[v.ID+":"+kind] == key {
			return nil
		}
		seen[v.ID+":"+kind] = key
		if time.Since(last[v.ID]) < 5*time.Second {
			return nil
		}
		last[v.ID] = time.Now()
		payload, _ := json.Marshal(struct {
			SchemaVersion int       `json:"schema_version"`
			AgentID       string    `json:"agent_id"`
			ManagedID     *string   `json:"managed_id"`
			Kind          string    `json:"kind"`
			Time          time.Time `json:"time"`
		}{1, v.ID, v.ManagedID, kind, e.Time})
		select {
		case queue <- payload:
		default:
		}
		return nil
	})
}
