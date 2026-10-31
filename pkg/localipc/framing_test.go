package localipc

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

type message struct {
	Schema   int     `json:"schema_version"`
	Children []child `json:"children"`
}
type child struct {
	Name string `json:"name"`
}
type fragment struct{ r io.Reader }

func (r fragment) Read(p []byte) (int, error) { return r.r.Read(p[:min(1, len(p))]) }
func TestFramingBoundsStrictFieldsAndFragments(t *testing.T) {
	var b bytes.Buffer
	want := message{1, []child{{"value"}}}
	if e := Write(&b, want); e != nil {
		t.Fatal(e)
	}
	var got message
	if e := Read(fragment{&b}, &got); e != nil || got.Children[0].Name != "value" {
		t.Fatal(got, e)
	}
	for _, body := range []string{`{"schema_version":1,"schema_version":1}`, `{"Schema_version":1}`, `{"children":[{"Name":"wrong case"}]}`, `{"unknown":1}`, `{"schema_version":1} {}`} {
		b.Reset()
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(body)))
		b.Write(header[:])
		b.WriteString(body)
		if Read(&b, &got) == nil {
			t.Fatalf("accepted invalid frame %s", body)
		}
	}
	for _, n := range []uint32{0, MaxBytes + 1} {
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], n)
		if Read(bytes.NewReader(header[:]), &got) == nil {
			t.Fatal("accepted length", n)
		}
	}
	for _, raw := range [][]byte{{0, 0}, {0, 0, 0, 10, '{'}} {
		if Read(bytes.NewReader(raw), &got) == nil {
			t.Fatal("accepted truncated frame")
		}
	}
}

func FuzzLocalFrame(f *testing.F) {
	var seed bytes.Buffer
	_ = Write(&seed, message{1, []child{{"seed"}}})
	f.Add(seed.Bytes())
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{0, 0, 0, 1, '{'})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxBytes+4 {
			return
		}
		var first message
		if Read(bytes.NewReader(data), &first) != nil {
			return
		}
		var out bytes.Buffer
		if e := Write(&out, first); e != nil {
			t.Fatal(e)
		}
		var second message
		if e := Read(&out, &second); e != nil {
			t.Fatal(e)
		}
		var a, b bytes.Buffer
		_ = Write(&a, first)
		_ = Write(&b, second)
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatal("local frame roundtrip changed data")
		}
	})
}
