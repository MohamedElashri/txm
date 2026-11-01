package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"
	"sync"

	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const MaxMessage = 1 << 20
const MaxRequests = 8192
const LegacyVersion = "2025-11-25"
const ModernVersion = "2026-07-28"

var methodName = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_./:-]{0,127}$`) })
var errTransportLimit = errors.New("MCP stdio framing or resource limit exceeded")

type requestState uint8

const (
	pending requestState = iota
	finished
	cancelledModern
	cancelledLegacy
)

type flow struct {
	mu     sync.Mutex
	used   map[jsonrpc.ID]requestState
	active int
	modern map[jsonrpc.ID]bool
	eof    bool
}

func (f *flow) input(line []byte) error {
	var fields map[string]json.RawMessage
	if nativeprotocol.Decode(line, &fields) != nil {
		return errTransportLimit
	}
	for key := range fields {
		if key != "jsonrpc" && key != "id" && key != "method" && key != "params" {
			return errTransportLimit
		}
	}
	msg, err := jsonrpc.DecodeMessage(line)
	if err != nil {
		return errTransportLimit
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok || !methodName().MatchString(req.Method) {
		return errTransportLimit
	}
	if (!req.IsCall() && !strings.HasPrefix(req.Method, "notifications/")) || (req.IsCall() && strings.HasPrefix(req.Method, "notifications/")) {
		return errTransportLimit
	}
	if raw, ok := fields["id"]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errTransportLimit
	}
	if req.IsCall() {
		switch id := req.ID.Raw().(type) {
		case string:
			if len(id) > 128 {
				return errTransportLimit
			}
		case int64:
			if id > 9007199254740991 || id < -9007199254740991 {
				return errTransportLimit
			}
		}
		if raw := fields["id"]; len(raw) > 0 && bytes.TrimSpace(raw)[0] != '"' {
			var id float64
			if json.Unmarshal(raw, &id) != nil || math.Trunc(id) != id {
				return errTransportLimit
			}
		}
	}
	var params struct {
		Meta      map[string]any `json:"_meta"`
		RequestID any            `json:"requestId"`
	}
	_ = json.Unmarshal(req.Params, &params)
	f.mu.Lock()
	defer f.mu.Unlock()
	if req.Method == "notifications/cancelled" {
		switch id := params.RequestID.(type) {
		case string:
			if len(id) > 128 {
				return errTransportLimit
			}
		case float64:
			if math.Trunc(id) != id || math.Abs(id) > 9007199254740991 {
				return errTransportLimit
			}
		default:
			return errTransportLimit
		}
		id, err := jsonrpc.MakeID(params.RequestID)
		if err == nil && id.IsValid() {
			if state, ok := f.used[id]; ok && state == pending {
				if f.modern[id] {
					f.used[id] = cancelledModern
				} else {
					f.used[id] = cancelledLegacy
				}
				f.active--
			}
		}
	}
	if req.IsCall() {
		if _, exists := f.used[req.ID]; exists || f.active >= 16 || len(f.used) >= MaxRequests {
			return errTransportLimit
		}
		f.used[req.ID] = pending
		f.active++
		if f.modern == nil {
			f.modern = map[jsonrpc.ID]bool{}
		}
		f.modern[req.ID] = params.Meta[sdk.MetaKeyProtocolVersion] == ModernVersion
	}
	return nil
}

type frameReader struct {
	reader *bufio.Reader
	closer io.Closer
	flow   *flow
	buffer []byte
	onEOF  func()
}

func (r *frameReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.buffer) == 0 {
		var line []byte
		for {
			part, err := r.reader.ReadSlice('\n')
			if len(line)+len(part) > MaxMessage {
				return 0, errTransportLimit
			}
			line = append(line, part...)
			if err == bufio.ErrBufferFull {
				continue
			}
			if err != nil {
				if err == io.EOF && len(line) == 0 {
					r.flow.mu.Lock()
					r.flow.eof = true
					r.flow.mu.Unlock()
					if r.onEOF != nil {
						r.onEOF()
					}
					return 0, io.EOF
				}
				return 0, errTransportLimit
			}
			break
		}
		if err := r.flow.input(line); err != nil {
			return 0, err
		}
		r.buffer = line
	}
	n := copy(p, r.buffer)
	r.buffer = r.buffer[n:]
	return n, nil
}
func (r *frameReader) Close() error { return r.closer.Close() }

type frameWriter struct {
	writer io.WriteCloser
	flow   *flow
}

func (w *frameWriter) Write(p []byte) (int, error) {
	originalSize := len(p)
	w.flow.mu.Lock()
	ended := w.flow.eof
	w.flow.mu.Unlock()
	if ended {
		return originalSize, nil
	}
	if len(p) > MaxMessage {
		return 0, errTransportLimit
	}
	msg, err := jsonrpc.DecodeMessage(bytes.TrimSpace(p))
	if err != nil {
		return 0, errTransportLimit
	}
	if req, ok := msg.(*jsonrpc.Request); ok && req.IsCall() {
		return 0, errTransportLimit
	}
	if response, ok := msg.(*jsonrpc.Response); ok {
		// SDK 1.7 advertises its entire compiled version list in this error,
		// even when the transport narrowed discovery. Keep refusal truthful.
		var protocolError *jsonrpc.Error
		if errors.As(response.Error, &protocolError) && protocolError.Code == sdk.CodeUnsupportedProtocolVersion {
			var data sdk.UnsupportedProtocolVersionData
			if json.Unmarshal(protocolError.Data, &data) != nil {
				return 0, errTransportLimit
			}
			data.Supported = []string{LegacyVersion, ModernVersion}
			copied := *protocolError
			copied.Data, _ = json.Marshal(data)
			response.Error = &copied
			encoded, e := jsonrpc.EncodeMessage(response)
			if e != nil {
				return 0, e
			}
			p = append(encoded, '\n')
			if len(p) > MaxMessage {
				return 0, errTransportLimit
			}
		}
		w.flow.mu.Lock()
		defer w.flow.mu.Unlock()
		if w.flow.eof {
			return originalSize, nil
		}
		state, known := w.flow.used[response.ID]
		if !known || state == finished {
			return 0, errTransportLimit
		}
		if state == cancelledModern {
			return originalSize, nil
		}
		// Serialize publication with cancellation receipt. A response already
		// being published precedes cancellation; a received cancel suppresses it.
		n, err := w.writer.Write(p)
		if err == nil && n != len(p) {
			err = io.ErrShortWrite
		}
		if err == nil {
			if state == pending {
				w.flow.active--
			}
			w.flow.used[response.ID] = finished
			return originalSize, nil
		}
		return n, err
	}
	return w.writer.Write(p)
}
func (w *frameWriter) Close() error { return w.writer.Close() }

type stdioTransport struct{ sdk.IOTransport }

func (t *stdioTransport) SupportsProtocolVersion(v string) bool {
	return v == LegacyVersion || v == ModernVersion
}
func (t *stdioTransport) Connect(ctx context.Context) (sdk.Connection, error) {
	return t.IOTransport.Connect(ctx)
}

func transport(input io.ReadCloser, output io.WriteCloser) *stdioTransport {
	f := &flow{used: map[jsonrpc.ID]requestState{}}
	return &stdioTransport{IOTransport: sdk.IOTransport{
		Reader: &frameReader{reader: bufio.NewReaderSize(input, 4096), closer: input, flow: f},
		Writer: &frameWriter{writer: output, flow: f},
	}}
}
