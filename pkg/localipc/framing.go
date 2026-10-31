// Package localipc provides bounded one-request local JSON framing. Callers own
// peer authentication, instance binding and finite connection deadlines.
package localipc

import (
	"encoding/binary"
	"encoding/json"
	"io"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
)

const MaxBytes = 1 << 20

func Read(r io.Reader, into any) error {
	var header [4]byte
	if _, e := io.ReadFull(r, header[:]); e != nil {
		return e
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 || n > MaxBytes {
		return core.Failure("protocol_error", "invalid local frame length")
	}
	body := make([]byte, int(n))
	if _, e := io.ReadFull(r, body); e != nil {
		return e
	}
	if e := nativeprotocol.Decode(body, into); e != nil {
		return core.Failure("protocol_error", "invalid local message")
	}
	return nil
}
func Write(w io.Writer, value any) error {
	body, e := json.Marshal(value)
	if e != nil {
		return e
	}
	if len(body) == 0 || len(body) > MaxBytes {
		return core.Failure("protocol_error", "local response exceeds limit")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(body)))
	for _, data := range [][]byte{header[:], body} {
		for len(data) > 0 {
			n, e := w.Write(data)
			if e != nil {
				return e
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			data = data[n:]
		}
	}
	return nil
}
