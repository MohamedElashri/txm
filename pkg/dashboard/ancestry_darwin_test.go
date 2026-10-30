package dashboard

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func TestDarwinArgumentsExcludeEnvironmentAndRejectTruncation(t *testing.T) {
	id := strings.Repeat("a", 32)
	args := []string{"/tmp/renamed", "server", "--instance", id, "", "sample"}
	raw := binary.NativeEndian.AppendUint32(nil, uint32(len(args)))
	raw = append(raw, []byte("/tmp/renamed\x00\x00\x00"+strings.Join(args, "\x00")+"\x00PRIVATE_ENV=secret\x00")...)
	got, ok := darwinArguments(raw)
	if !ok || !reflect.DeepEqual(got, args) || serverInstance(got) != id {
		t.Fatal(got, ok)
	}
	for _, bad := range [][]byte{nil, raw[:3], raw[:10], binary.NativeEndian.AppendUint32(nil, 0), binary.NativeEndian.AppendUint32(nil, 4097)} {
		if _, ok := darwinArguments(bad); ok {
			t.Fatal("malformed kernel arguments accepted")
		}
	}
}
