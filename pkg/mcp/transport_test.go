package mcp

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func TestFrameLimitsAndCancellation(t *testing.T) {
	for _, line := range []string{
		`{"jsonrpc":"2.0","id":null,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":1,"id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":1.5,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":9007199254740992,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":1,"result":{}}`,
		`{"jsonrpc":"2.0","method":"tools/call"}`,
		`{"jsonrpc":"2.0","id":1,"method":"notifications/cancelled"}`,
		`{"jsonrpc":"2.0","id":1,"method":"ping","unknown":true}`,
	} {
		f := &flow{used: map[jsonrpc.ID]requestState{}}
		if f.input([]byte(line)) == nil {
			t.Fatalf("accepted invalid frame %s", line)
		}
	}
	f := &flow{used: map[jsonrpc.ID]requestState{}}
	for i := 1; i <= 16; i++ {
		line := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"ping","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`, i)
		if err := f.input([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if f.input([]byte(`{"jsonrpc":"2.0","id":17,"method":"ping"}`)) == nil {
		t.Fatal("concurrency exceeded")
	}
	if err := f.input([]byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`)); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	w := frameWriter{writer: nopWriteCloser{&output}, flow: f}
	response := []byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n")
	if _, err := w.Write(response); err != nil || output.Len() != 0 {
		t.Fatalf("modern cancelled response emitted: %v", err)
	}
	if f.active != 15 {
		t.Fatal("cancellation did not release slot")
	}
	if f.input([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)) == nil {
		t.Fatal("cancelled request ID reused")
	}
	r := transport(io.NopCloser(strings.NewReader(strings.Repeat("x", MaxMessage)+"\n")), nopWriteCloser{&output})
	if _, err := io.ReadAll(r.Reader); err == nil {
		t.Fatal("oversize frame accepted")
	}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func FuzzMCPFrame(f *testing.F) {
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	f.Add([]byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > MaxMessage {
			t.Skip()
		}
		flow := &flow{used: map[jsonrpc.ID]requestState{}}
		_ = flow.input(b)
	})
}

func TestScopedCursorAndEnvelopeBudget(t *testing.T) {
	s := &Server{}
	items := []string{"a", "b", "c"}
	first, err := page(s, "agents", ListInput{Limit: 1}, items, func(s string) string { return s })
	if err != nil || !first.Partial || first.NextCursor == "" {
		t.Fatalf("first page: %v %+v", err, first)
	}
	if _, err = page(s, "tasks", ListInput{Limit: 1, Cursor: first.NextCursor}, items, func(s string) string { return s }); err == nil {
		t.Fatal("cross tool cursor")
	}
	other := &Server{key: [32]byte{1}}
	if _, err = page(other, "agents", ListInput{Limit: 1, Cursor: first.NextCursor}, items, func(s string) string { return s }); err == nil {
		t.Fatal("cross scope cursor")
	}
	if _, err = page(s, "agents", ListInput{Limit: 1, Cursor: first.NextCursor}, []string{"a", "changed", "c"}, func(s string) string { return s }); err == nil {
		t.Fatal("changed inventory cursor")
	}
	if _, err = page(s, "agents", ListInput{}, []string{strings.Repeat("x", MaxMessage)}, func(string) string { return "a" }); err == nil {
		t.Fatal("oversize output")
	}
}
