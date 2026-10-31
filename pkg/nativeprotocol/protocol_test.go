package nativeprotocol

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"
)

type fragments struct {
	b     []byte
	chunk int
}

func (r *fragments) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := min(len(p), len(r.b), r.chunk)
	copy(p, r.b[:n])
	r.b = r.b[n:]
	return n, nil
}

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(b []byte) (int, error) { return w.Buffer.Write(b[:min(2, len(b))]) }
func TestFramingGoldenFragmentationAndCoalescing(t *testing.T) {
	f, _ := JSON(Hello, 1, Greeting{"observer"})
	var b bytes.Buffer
	if e := Write(&b, f); e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(b.Bytes()[:HeaderSize]) != "54584d3100010100000000000000000100000013" {
		t.Fatalf("header %x", b.Bytes())
	}
	for chunk := 1; chunk <= b.Len(); chunk++ {
		r := &fragments{append(append([]byte{}, b.Bytes()...), b.Bytes()...), chunk}
		for range 2 {
			got, e := Read(r)
			if e != nil || got.Type != Hello || !bytes.Equal(got.Payload, f.Payload) {
				t.Fatalf("fragment %d: %v", chunk, e)
			}
		}
	}
	var short shortWriter
	if e := Write(&short, f); e != nil || !bytes.Equal(short.Bytes(), b.Bytes()) {
		t.Fatal("short writes corrupted frame", e)
	}
	for cut := 1; cut < b.Len(); cut++ {
		if _, e := Read(bytes.NewReader(b.Bytes()[:cut])); e == nil {
			t.Fatalf("truncation accepted %d", cut)
		}
	}
	for _, offset := range []int{0, 5, 6, 7, 15, 16} {
		copyBytes := append([]byte{}, b.Bytes()...)
		copyBytes[offset] = 255
		if _, e := Read(bytes.NewReader(copyBytes)); e == nil {
			t.Fatalf("invalid header accepted offset %d", offset)
		}
	}
}
func TestStrictJSON(t *testing.T) {
	for _, b := range []string{`null`, `[]`, `{"role":"control","role":"observer"}`, `{"other":1}`, `{"role":"observer","Role":"control"}`, `{"role":"control"} {}`, "{\"role\":\"\xff\"}"} {
		var h Greeting
		if Decode([]byte(b), &h) == nil {
			t.Fatalf("accepted %q", b)
		}
	}
}
func FuzzNativeFrame(f *testing.F) {
	hello, _ := JSON(Hello, 1, Greeting{"observer"})
	var b bytes.Buffer
	_ = Write(&b, hello)
	f.Add(b.Bytes())
	f.Add([]byte("TXM1"))
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 2*MaxPayload {
			return
		}
		r := bytes.NewReader(input)
		for range 16 {
			frame, e := Read(r)
			if e != nil {
				return
			}
			var out bytes.Buffer
			if e = Write(&out, frame); e != nil {
				t.Fatal(e)
			}
			again, e := Read(&out)
			if e != nil || !bytes.Equal(frame.Payload, again.Payload) {
				t.Fatal("round trip", e)
			}
		}
	})
}
