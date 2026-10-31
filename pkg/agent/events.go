package agent

import (
	"encoding/json"
	"time"
)

type Event struct {
	SchemaVersion    int           `json:"schema_version"`
	CoordinatorEpoch string        `json:"coordinator_epoch"`
	Sequence         uint64        `json:"sequence"`
	Time             time.Time     `json:"time"`
	ManagedID        *string       `json:"managed_id"`
	AgentID          *string       `json:"agent_id"`
	Kind             string        `json:"kind"`
	Source           string        `json:"source"`
	Metadata         EventMetadata `json:"metadata"`
}
type EventMetadata struct {
	Reason string `json:"reason,omitempty"`
	Agents []View `json:"agents,omitempty"`
	Agent  *View  `json:"agent,omitempty"`
}
type subscription struct {
	id       string
	queue    chan Event
	resync   chan struct{}
	overflow bool
	reason   string
}

// All hub methods run under the coordinator's state gate. Queue sends never
// block a mutation, backend operation or another subscriber.
type eventHub struct {
	epoch    string
	sequence uint64
	subs     map[*subscription]bool
	last     map[string]string
}

func newHub(epoch string) *eventHub {
	return &eventHub{epoch: epoch, subs: map[*subscription]bool{}, last: map[string]string{}}
}
func (h *eventHub) event(kind, source string, v *View, meta EventMetadata) Event {
	h.sequence++
	e := Event{SchemaVersion: 1, CoordinatorEpoch: h.epoch, Sequence: h.sequence, Time: time.Now().UTC(), Kind: kind, Source: source, Metadata: meta}
	if v != nil {
		e.AgentID = &v.ID
		e.ManagedID = v.ManagedID
	}
	return e
}
func (h *eventHub) emit(kind, source string, v *View) {
	e := h.event(kind, source, v, EventMetadata{Agent: v})
	for sub := range h.subs {
		if sub.overflow || (sub.id != "" && (v == nil || sub.id != v.ID) && kind != "heartbeat") {
			continue
		}
		select {
		case sub.queue <- e:
		default:
			sub.overflow = true
			for {
				select {
				case <-sub.queue:
					continue
				default:
					goto drained
				}
			}
		drained:
			select {
			case sub.resync <- struct{}{}:
			default:
			}
		}
	}
}
func (h *eventHub) track(v View) {
	// Observation timestamps renew on every probe. Emit transitions, not a
	// stream of identical running observations.
	sig, _ := json.Marshal(struct {
		Lifecycle, Attention, AttentionSource, Name string
		Turn                                        *time.Time
		Receipt                                     any
	}{v.Lifecycle, v.Attention, v.AttentionSource, v.Locator.Name, v.TurnCompleteAt, v.Receipt})
	if h.last[v.ID] != string(sig) {
		h.last[v.ID] = string(sig)
		h.emit("agent_changed", v.Source, &v)
	}
}
func (h *eventHub) add(id string) *subscription {
	s := &subscription{id: id, queue: make(chan Event, 16), resync: make(chan struct{}, 1)}
	h.subs[s] = true
	return s
}
