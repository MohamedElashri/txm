package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestOpenCodeV2ProjectionIsScopedBoundedAndNotTurnOrExitEvidence(t *testing.T) {
	o, err := NewOpenCodeV2Observer("ses_A")
	if err != nil {
		t.Fatal(err)
	}
	event := func(id, session, kind string, seq int) []byte {
		return []byte(fmt.Sprintf(`{"id":%q,"created":12345,"type":%q,"durable":{"aggregateID":"ses_A","seq":%d,"version":1},"data":{"sessionID":%q,"error":{"message":"PRIVATE"},"resources":["PRIVATE_PATH"]},"metadata":{"prompt":"PRIVATE"}}`, id, kind, seq, session))
	}
	if got, err := o.Accept(event("evt_unrelated", "ses_B", "session.execution.started", 1)); err != nil || got != nil {
		t.Fatal("observed another session", got, err)
	}
	got, err := o.Accept(event("evt_1", "ses_A", "session.execution.started", 1))
	if err != nil || got.Ordering != "aggregate_sequence" {
		t.Fatal(got, err)
	}
	if _, err = o.Accept(event("evt_2", "ses_A", "session.execution.succeeded", 1)); err == nil {
		t.Fatal("accepted replayed sequence")
	}
	got, err = o.Accept(event("evt_3", "ses_A", "session.execution.succeeded", 2))
	if err != nil || got.TurnOutcome != "unverified" {
		t.Fatal("invented provider turn identity", got, err)
	}
	encoded, _ := json.Marshal(got)
	for _, secret := range []string{"PRIVATE", "exit_code", "receipt", "needs_input"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("content or inferred state leaked", secret)
		}
	}
	got, err = o.Accept([]byte(`{"id":"evt_4","created":12346,"type":"permission.asked","data":{"sessionID":"ses_A","id":"request","action":"shell","resources":["PRIVATE"]}}`))
	if err != nil || got.Sequence != nil || got.Ordering != "unverified" || got.TurnOutcome != "unverified" {
		t.Fatal(got, err)
	}
	for i := 0; i < 100; i++ {
		_, err = o.Accept([]byte(fmt.Sprintf(`{"id":"evt_idle-%d","created":12347,"type":"session.idle","data":{"sessionID":"ses_A"}}`, i)))
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(o.recent) > 64 {
		t.Fatal("unbounded deduplication")
	}
	if validProvider("opencode", "opencode v2.2.5") {
		t.Fatal("projection fixtures enabled an unverified runtime")
	}
}

func TestOpenCodeV2RejectsV1MalformedAndMissingDurableData(t *testing.T) {
	for _, body := range []string{
		`{"type":"session.idle","properties":{"sessionID":"ses_A"}}`,
		`{"id":"evt_event","id":"evt_duplicate","created":1,"type":"session.idle","data":{"sessionID":"ses_A"}}`,
		`{"id":"evt_event","created":1,"type":"session.execution.succeeded","data":{"sessionID":"ses_A"}}`,
		`{"id":"evt_event","created":1,"type":"session.execution.started","durable":{"aggregateID":"ses_A","seq":-1,"version":1},"data":{"sessionID":"ses_A"}}`,
		strings.Repeat("x", 65537),
	} {
		o, _ := NewOpenCodeV2Observer("ses_A")
		if _, err := o.Accept([]byte(body)); err == nil {
			t.Fatal("accepted invalid v2 envelope")
		}
		if o.sequenced || len(o.recent) != 0 {
			t.Fatal("invalid event changed cursor")
		}
	}
}
