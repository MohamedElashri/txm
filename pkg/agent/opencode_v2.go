package agent

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
)

// OpenCodeV2Metadata projects the documented v2 public-event envelope. This is
// deliberately not a canonical provider activation or a turn/process receipt.
// Runtime/plugin loading is unverified and remains disabled in validProvider.
type OpenCodeV2Metadata struct {
	EventID         string      `json:"event_id"`
	SessionID       string      `json:"session_id"`
	Event           string      `json:"event"`
	ProviderCreated json.Number `json:"provider_created"`
	AggregateID     string      `json:"aggregate_id,omitempty"`
	Sequence        *uint64     `json:"sequence"`
	Ordering        string      `json:"ordering"`
	TurnOutcome     string      `json:"turn_outcome"`
}

// OpenCodeV2Observer is session-scoped and bounded. Durable events are ordered
// only within their declared aggregate. Non-durable permission/idle events have
// unverified ordering and can never become confirmed needs-input/completion.
type OpenCodeV2Observer struct {
	session   string
	aggregate string
	sequence  uint64
	sequenced bool
	recent    []string
}

func NewOpenCodeV2Observer(session string) (*OpenCodeV2Observer, error) {
	if !providerSession().MatchString(session) || !strings.HasPrefix(session, "ses") {
		return nil, core.Failure("invalid_target", "invalid OpenCode session scope")
	}
	return &OpenCodeV2Observer{session: session}, nil
}

func (o *OpenCodeV2Observer) Accept(data []byte) (*OpenCodeV2Metadata, error) {
	bad := func() (*OpenCodeV2Metadata, error) {
		return nil, core.Failure("protocol_error", "invalid OpenCode v2 event metadata")
	}
	if o == nil || !providerSession().MatchString(o.session) || len(data) > 64<<10 {
		return bad()
	}
	var raw map[string]json.RawMessage
	if nativeprotocol.Decode(data, &raw) != nil {
		return bad()
	}
	var id, kind string
	var payload map[string]json.RawMessage
	var session string
	if json.Unmarshal(raw["id"], &id) != nil || !providerSession().MatchString(id) || !strings.HasPrefix(id, "evt_") || json.Unmarshal(raw["type"], &kind) != nil || json.Unmarshal(raw["data"], &payload) != nil || payload == nil || json.Unmarshal(payload["sessionID"], &session) != nil || !providerSession().MatchString(session) || !strings.HasPrefix(session, "ses") {
		return bad()
	}
	if session != o.session {
		return nil, nil
	}
	switch kind {
	case "session.execution.started", "session.execution.succeeded", "session.execution.failed", "session.execution.interrupted", "session.idle", "session.status", "permission.asked", "permission.replied":
	default:
		return nil, core.Failure("unsupported", "OpenCode v2 event is not observed")
	}
	var created json.Number
	createdRaw := bytes.TrimSpace(raw["created"])
	if len(createdRaw) == 0 || createdRaw[0] == '"' || json.Unmarshal(createdRaw, &created) != nil {
		return bad()
	}
	timestamp, err := strconv.ParseFloat(string(created), 64)
	if err != nil || math.IsNaN(timestamp) || math.IsInf(timestamp, 0) {
		return bad()
	}
	result := &OpenCodeV2Metadata{EventID: id, SessionID: session, Event: kind, ProviderCreated: created, Ordering: "unverified", TurnOutcome: "unverified"}
	if strings.HasPrefix(kind, "session.execution.") {
		var durable struct {
			Aggregate string  `json:"aggregateID"`
			Sequence  *uint64 `json:"seq"`
			Version   *int    `json:"version"`
		}
		if nativeprotocol.Decode(raw["durable"], &durable) != nil || !providerSession().MatchString(durable.Aggregate) || durable.Sequence == nil || *durable.Sequence > 9007199254740991 || durable.Version == nil || *durable.Version != 1 {
			return bad()
		}
		if o.sequenced && (durable.Aggregate != o.aggregate || *durable.Sequence <= o.sequence) {
			return nil, core.Failure("conflict", "stale or unrelated OpenCode event aggregate")
		}
		result.AggregateID, result.Sequence, result.Ordering = durable.Aggregate, durable.Sequence, "aggregate_sequence"
	}
	for _, seen := range o.recent {
		if seen == id {
			return nil, core.Failure("conflict", "replayed OpenCode event")
		}
	}
	// Commit cursor changes only after the complete projection passes validation.
	if result.Sequence != nil {
		o.aggregate, o.sequence, o.sequenced = result.AggregateID, *result.Sequence, true
	}
	if len(o.recent) == 64 {
		copy(o.recent, o.recent[1:])
		o.recent = o.recent[:63]
	}
	o.recent = append(o.recent, id)
	return result, nil
}
