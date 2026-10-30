// Package nativeprotocol implements the bounded, versioned native wire format.
package nativeprotocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	Version           = 1
	HeaderSize        = 20
	MaxPayload        = 1 << 20
	MaxData           = 16 << 10
	MaxConnections    = 32
	QueueBytes        = 256 << 10
	QueueFrames       = 64
	RequestTimeout    = 5 * time.Second
	WriteTimeout      = 2 * time.Second
	HeartbeatInterval = 30 * time.Second
	HeartbeatTimeout  = 5 * time.Second
)

type Type byte

const (
	Hello Type = iota + 1
	HelloOK
	Request
	Response
	Data
	Resize
	Close
	ErrorFrame
	Ping
	Pong
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string           { return e.Code + ": " + e.Message }
func Failure(code, message string) error { return &Error{code, message} }

type Frame struct {
	Type    Type
	ID      uint64
	Payload []byte
}

func validate(t Type, id uint64, size uint32) error {
	if t < Hello || t > Pong || size > MaxPayload {
		return Failure("protocol_error", "invalid frame type or length")
	}
	switch t {
	case Hello, HelloOK:
		if id != 1 || size == 0 {
			return Failure("protocol_error", "invalid handshake frame")
		}
	case Request, Response:
		if id < 2 || size == 0 {
			return Failure("protocol_error", "invalid request/response frame")
		}
	case Data:
		if id != 0 || size == 0 || size > MaxData {
			return Failure("protocol_error", "invalid data frame")
		}
	case Resize:
		if id < 2 || size != 4 {
			return Failure("protocol_error", "invalid resize frame")
		}
	case Close:
		if id != 0 || size != 0 {
			return Failure("protocol_error", "invalid close frame")
		}
	case Ping, Pong:
		if id == 0 || size != 0 {
			return Failure("protocol_error", "invalid heartbeat frame")
		}
	case ErrorFrame:
		if size == 0 {
			return Failure("protocol_error", "empty error frame")
		}
	}
	return nil
}
func Read(r io.Reader) (Frame, error) {
	var h [HeaderSize]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return Frame{}, err
	}
	if string(h[:4]) != "TXM1" {
		return Frame{}, Failure("protocol_error", "invalid frame magic")
	}
	if binary.BigEndian.Uint16(h[4:6]) != Version {
		return Frame{}, Failure("unsupported_version", "native protocol version mismatch")
	}
	if h[7] != 0 {
		return Frame{}, Failure("protocol_error", "unknown frame flags")
	}
	t, id, n := Type(h[6]), binary.BigEndian.Uint64(h[8:16]), binary.BigEndian.Uint32(h[16:20])
	if err := validate(t, id, n); err != nil {
		return Frame{}, err
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Frame{}, err
	}
	return Frame{t, id, payload}, nil
}
func Write(w io.Writer, f Frame) error {
	if len(f.Payload) > MaxPayload {
		return Failure("limit_exceeded", "payload too large")
	}
	if err := validate(f.Type, f.ID, uint32(len(f.Payload))); err != nil {
		return err
	}
	var h [HeaderSize]byte
	copy(h[:4], "TXM1")
	binary.BigEndian.PutUint16(h[4:6], Version)
	h[6] = byte(f.Type)
	binary.BigEndian.PutUint64(h[8:16], f.ID)
	binary.BigEndian.PutUint32(h[16:20], uint32(len(f.Payload)))
	for _, b := range [][]byte{h[:], f.Payload} {
		for len(b) > 0 {
			n, err := w.Write(b)
			if err != nil {
				return err
			}
			if n <= 0 || n > len(b) {
				return io.ErrShortWrite
			}
			b = b[n:]
		}
	}
	return nil
}
func JSON(t Type, id uint64, value any) (Frame, error) {
	b, err := json.Marshal(value)
	return Frame{t, id, b}, err
}

// Decode rejects duplicate keys (including nested objects), excessive nesting,
// trailing values, invalid UTF-8, unknown fields and non-object roots.
func Decode(b []byte, value any) error {
	typ := reflect.TypeOf(value)
	if typ == nil || typ.Kind() != reflect.Pointer || reflect.ValueOf(value).IsNil() {
		return Failure("protocol_error", "JSON destination must be a non-nil pointer")
	}
	if len(b) > MaxPayload || !utf8.Valid(b) || len(bytes.TrimSpace(b)) == 0 || bytes.TrimSpace(b)[0] != '{' {
		return Failure("protocol_error", "invalid JSON object")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := tokens(dec, 0); err != nil {
		return Failure("protocol_error", err.Error())
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Failure("protocol_error", "trailing JSON value")
	}
	if err := exactFields(b, reflect.TypeOf(value)); err != nil {
		return Failure("protocol_error", "unknown JSON field")
	}
	dec = json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return Failure("protocol_error", "invalid JSON fields")
	}
	return nil
}

// encoding/json otherwise accepts case-insensitive aliases such as role/Role.
// Match tags exactly, including nested error objects; RawMessage is opaque data.
func exactFields(raw []byte, typ reflect.Type) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == reflect.TypeFor[[]byte]() {
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) > 0 && (trimmed[0] == '"' || bytes.Equal(trimmed, []byte("null"))) {
			return nil
		}
		return fmt.Errorf("expected base64 string")
	}
	if (typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array) && !reflect.PointerTo(typ).Implements(reflect.TypeFor[json.Unmarshaler]()) {
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return err
		}
		for _, value := range values {
			if err := exactFields(value, typ.Elem()); err != nil {
				return err
			}
		}
		return nil
	}
	if typ.Kind() != reflect.Struct || reflect.PointerTo(typ).Implements(reflect.TypeFor[json.Unmarshaler]()) {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	allowed := map[string]reflect.Type{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			allowed[name] = f.Type
		}
	}
	for name, data := range fields {
		field, ok := allowed[name]
		if !ok {
			return fmt.Errorf("unknown field")
		}
		if err := exactFields(data, field); err != nil {
			return err
		}
	}
	return nil
}
func tokens(d *json.Decoder, depth int) error {
	if depth > 32 {
		return fmt.Errorf("JSON nesting limit")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate or invalid JSON key")
			}
			seen[key] = true
			if e = tokens(d, depth+1); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e := tokens(d, depth+1); e != nil {
				return e
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
	_, err = d.Token()
	return err
}

type Greeting struct {
	Role string `json:"role"`
}
type Identity struct {
	ProtocolVersion int       `json:"protocol_version"`
	InstanceID      string    `json:"instance_id"`
	Backend         string    `json:"backend"`
	Name            string    `json:"name"`
	Role            string    `json:"role"`
	ProcessState    string    `json:"process_state"`
	Capabilities    []string  `json:"capabilities"`
	ObservedAt      time.Time `json:"observed_at"`
}
type Operation struct {
	Op       string `json:"op"`
	Format   string `json:"format,omitempty"`
	Lines    int    `json:"lines,omitempty"`
	MaxBytes int    `json:"max_bytes,omitempty"`
	Data     []byte `json:"data,omitempty"`
}
type Reply struct {
	Data  json.RawMessage `json:"data"`
	Error *Error          `json:"error"`
}
type Status struct {
	InstanceID      string `json:"instance_id"`
	Name            string `json:"name"`
	ProcessState    string `json:"process_state"`
	AttachedClients int    `json:"attached_clients"`
}
type Snapshot struct {
	Type       string    `json:"type"`
	Encoding   string    `json:"encoding"`
	Content    []byte    `json:"content"`
	Lines      int       `json:"lines"`
	Truncated  bool      `json:"truncated"`
	ObservedAt time.Time `json:"observed_at"`
	Position   uint64    `json:"position"`
}
