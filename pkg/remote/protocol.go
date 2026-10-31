package remote

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const ProtocolVersion = 1

type Request struct {
	Schema    int    `json:"schema_version"`
	Protocol  int    `json:"protocol_version"`
	Op        string `json:"op"`
	HostID    string `json:"host_id"`
	Backend   string `json:"backend"`
	Name      string `json:"name"`
	BackendID string `json:"backend_id"`
	Instance  string `json:"instance_id"`
	ReadOnly  bool   `json:"read_only"`
}
type Data struct {
	Protocol int            `json:"protocol_version"`
	HostID   string         `json:"host_id"`
	Sessions []core.Session `json:"sessions"`
}

var tmuxID = regexp.MustCompile(`^\$[0-9]+$`)

func (q Request) Validate() error {
	if q.Schema != 1 || q.Protocol != ProtocolVersion {
		return core.Failure("unsupported_version", "remote schema/protocol mismatch")
	}
	if q.HostID != "" && !runtimepath.ValidID(q.HostID) {
		return core.Failure("invalid_target", "remote host requires full identity")
	}
	if q.Op == "probe" {
		if q.Backend != "" || q.Name != "" || q.BackendID != "" || q.Instance != "" || q.ReadOnly {
			return core.Failure("invalid_usage", "unexpected probe fields")
		}
		return nil
	}
	if !runtimepath.ValidID(q.HostID) || (q.Backend != "native" && q.Backend != "tmux") {
		return core.Failure("invalid_target", "remote request requires pinned host and native/tmux backend")
	}
	switch q.Op {
	case "list":
		if q.Name != "" || q.BackendID != "" || q.Instance != "" || q.ReadOnly {
			return core.Failure("invalid_usage", "unexpected list fields")
		}
	case "attach":
		if q.Name == "" || len(q.Name) > 512 || !utf8.ValidString(q.Name) || strings.ContainsRune(q.Name, 0) {
			return core.Failure("invalid_target", "invalid remote session name")
		}
		if q.Backend == "native" {
			if !runtimepath.ValidID(q.Instance) || q.BackendID != q.Instance {
				return core.Failure("invalid_target", "native attach requires exact launch instance")
			}
		} else if !tmuxID.MatchString(q.BackendID) || q.Instance != "" {
			return core.Failure("invalid_target", "tmux attach requires exact server-scoped ID")
		}
	default:
		return core.Failure("invalid_usage", "unknown remote operation")
	}
	return nil
}
func Encode(q Request) (string, error) {
	if e := q.Validate(); e != nil {
		return "", e
	}
	b, e := json.Marshal(q)
	if e != nil {
		return "", e
	}
	if len(b) > 4096 {
		return "", core.Failure("invalid_usage", "remote request exceeded limit")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func Decode(s string) (Request, error) {
	q := Request{}
	if len(s) > 8192 {
		return q, core.Failure("invalid_usage", "remote request exceeded limit")
	}
	b, e := base64.RawURLEncoding.Strict().DecodeString(s)
	if e != nil || nativeprotocol.Decode(b, &q) != nil {
		return q, core.Failure("protocol_error", "invalid remote request")
	}
	return q, q.Validate()
}

// Command contains only three fixed tokens and a URL-safe encoded request.
func Command(h Host, q Request) (string, error) {
	if !validHost(h) {
		return "", core.Failure("invalid_usage", "invalid remote host configuration")
	}
	encoded, e := Encode(q)
	if e != nil {
		return "", e
	}
	return h.Executable + " internal remote --request " + encoded, nil
}

// Execute validates local host identity before instantiating a backend.
func Execute(ctx context.Context, q Request) (Data, error) {
	d := Data{Protocol: ProtocolVersion, Sessions: []core.Session{}}
	if e := q.Validate(); e != nil {
		return d, e
	}
	id, e := runtimepath.HostID(ctx)
	if e != nil {
		return d, e
	}
	d.HostID = id
	if q.HostID != "" && q.HostID != id {
		return d, core.Failure("conflict", "remote host identity does not match pin")
	}
	if q.Op == "probe" {
		return d, nil
	}
	b, e := backend.NewBackend(q.Backend, ctx)
	if e != nil {
		return d, e
	}
	if q.Op == "attach" {
		s := core.Session{Locator: core.SessionLocator{HostID: id, Backend: q.Backend, Name: q.Name}, BackendID: core.String(q.BackendID)}
		if q.Instance != "" {
			s.InstanceID = core.String(q.Instance)
		}
		return d, backend.AttachObserved(ctx, b, s, q.ReadOnly)
	}
	found, e := b.Sessions(ctx)
	if e != nil {
		return d, e
	}
	for i := range found {
		found[i].Locator.HostID = id
	}
	d.Sessions = found
	return d, nil
}
func validateData(d Data, q Request) error {
	if d.Protocol != ProtocolVersion {
		return core.Failure("unsupported_version", "remote protocol mismatch")
	}
	if !runtimepath.ValidID(d.HostID) || (q.HostID != "" && d.HostID != q.HostID) {
		return core.Failure("conflict", "remote host identity mismatch")
	}
	if d.Sessions == nil || len(d.Sessions) > 4096 || (q.Op == "probe" && len(d.Sessions) != 0) {
		return core.Failure("protocol_error", "invalid remote sessions")
	}
	seen := map[string]bool{}
	names := map[string]bool{}
	for _, s := range d.Sessions {
		if s.Locator.HostID != d.HostID || s.Locator.Backend != q.Backend || s.Locator.Name == "" || len(s.Locator.Name) > 512 || strings.ContainsRune(s.Locator.Name, 0) || s.BackendID == nil || seen[*s.BackendID] || names[s.Locator.Name] {
			return core.Failure("protocol_error", "remote session identity mismatch or collision")
		}
		if q.Backend == "native" {
			if s.InstanceID == nil || !runtimepath.ValidID(*s.InstanceID) || *s.BackendID != *s.InstanceID {
				return core.Failure("protocol_error", "invalid remote native instance")
			}
		} else if !tmuxID.MatchString(*s.BackendID) {
			return core.Failure("protocol_error", "invalid remote tmux ID")
		}
		seen[*s.BackendID] = true
		names[s.Locator.Name] = true
	}
	return nil
}
