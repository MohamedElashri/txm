package mcp

import (
	"bytes"
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
	"io"
	"strings"
	"testing"
	"time"
)

func TestCleanEOFAndMalformedEOFRemainDistinct(t *testing.T) {
	for _, input := range []string{"", "not-json\n", "{\"jsonrpc\":\"2.0\"}"} {
		s, e := New(core.ReadScope{}, "fixture")
		if e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		var output bytes.Buffer
		e = s.Run(ctx, io.NopCloser(strings.NewReader(input)), nopWriteCloser{&output})
		cancel()
		if input == "" && e != nil {
			t.Fatal("clean EOF reported failure", e)
		}
		if input != "" && e == nil {
			t.Fatal("malformed/truncated framing treated as clean EOF")
		}
		if output.Len() != 0 {
			t.Fatal("unexpected stdout on EOF")
		}
	}
}
