// Package mcp exposes only explicit scoped operations over local stdio.
package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/control"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	sdk      *sdk.Server
	reader   *control.Reader
	key      [32]byte
	requests chan struct{}
	waits    chan struct{}
	creator  *control.Creator
}

type Output struct {
	SchemaVersion int                  `json:"schema_version"`
	Data          any                  `json:"data"`
	Partial       bool                 `json:"partial"`
	NextCursor    string               `json:"next_cursor,omitempty"`
	Error         *core.OperationError `json:"error,omitempty"`
}
type ListInput struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor,omitempty"`
}
type GetInput struct {
	ID string `json:"id"`
}
type WaitInput struct {
	ID        string `json:"id"`
	Until     string `json:"until"`
	TimeoutMS int    `json:"timeout_ms"`
}

func New(scope core.ReadScope, version string) (*Server, error) {
	reader, err := control.NewReader(scope)
	if err != nil {
		return nil, err
	}
	s := &Server{reader: reader, requests: make(chan struct{}, 16), waits: make(chan struct{}, 4)}
	if _, err = rand.Read(s.key[:]); err != nil {
		return nil, core.Failure("invalid_config", "MCP cursor key unavailable")
	}
	s.sdk = sdk.NewServer(&sdk.Implementation{Name: "txm", Version: version}, &sdk.ServerOptions{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Capabilities: &sdk.ServerCapabilities{Tools: &sdk.ToolCapabilities{}}, PageSize: 16,
		Instructions: "Use only configured scopes. Session tools report managed registrations, not live backend attachment proof. Stop, cleanup and merge require a human CLI handoff. Provider fixture validation is not live support.",
	})
	s.sdk.AddReceivingMiddleware(eraMiddleware())
	idSchema := map[string]any{"type": "string", "pattern": "^[a-f0-9]{32}$"}
	listSchema := schema(map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 256, "default": 128}, "cursor": map[string]any{"type": "string", "maxLength": 512}}, nil)
	getSchema := schema(map[string]any{"id": idSchema}, []string{"id"})
	addRead(s, "list_agents", "List only authorized existing agents; never start or repair a coordinator.", listSchema, false, func(ctx context.Context, in ListInput) (Output, error) {
		v, e := s.reader.Agents(ctx)
		if e != nil {
			return Output{}, e
		}
		return page(s, "list_agents", in, v, func(v agent.PublicView) string { return v.ID })
	})
	addRead(s, "get_agent", "Read one exact authorized managed ID from existing metadata and receipts.", getSchema, false, func(ctx context.Context, in GetInput) (Output, error) {
		v, e := s.reader.Agent(ctx, in.ID)
		return Output{Data: v}, e
	})
	addRead(s, "list_sessions", "List authorized managed registrations; backend session liveness and attachment are unverified.", listSchema, false, func(ctx context.Context, in ListInput) (Output, error) {
		v, e := s.reader.Sessions(ctx)
		if e != nil {
			return Output{}, e
		}
		return page(s, "list_sessions", in, v, func(v control.SessionView) string { return v.AgentID })
	})
	addRead(s, "get_session", "Read a managed registration by exact agent ID; does not attach or capture output.", getSchema, false, func(ctx context.Context, in GetInput) (Output, error) {
		v, e := s.reader.Session(ctx, in.ID)
		return Output{Data: v}, e
	})
	addRead(s, "list_tasks", "Read scoped task metadata and pinned exit proof without writer locks or coordinator startup.", listSchema, false, func(ctx context.Context, in ListInput) (Output, error) {
		v, e := s.reader.Tasks(ctx)
		if e != nil {
			return Output{}, e
		}
		return page(s, "list_tasks", in, v, func(v control.TaskView) string { return v.ID })
	})
	addRead(s, "get_task", "Read one owned task in the startup read scope; no setup, cleanup or Git commands.", getSchema, false, func(ctx context.Context, in GetInput) (Output, error) {
		v, e := s.reader.Task(ctx, in.ID)
		return Output{Data: v}, e
	})
	waitSchema := schema(map[string]any{"id": idSchema, "until": map[string]any{"type": "string", "enum": []string{"exited", "idle", "needs_input", "turn_complete"}, "default": "exited"}, "timeout_ms": map[string]any{"type": "integer", "minimum": 1, "maximum": 60000, "default": 30000}}, []string{"id"})
	addRead(s, "wait_agent", "Wait at most 60s for scoped evidence. Missing/unknown state is never process success; no service is started.", waitSchema, true, func(ctx context.Context, in WaitInput) (Output, error) {
		v, e := s.reader.Wait(ctx, in.ID, in.Until, time.Duration(in.TimeoutMS)*time.Millisecond)
		return Output{Data: v}, e
	})
	addRead(s, "get_capabilities", "Report this server's bounded local capabilities without provider execution or filesystem discovery.", schema(map[string]any{}, nil), false, func(context.Context, struct{}) (Output, error) {
		return Output{Data: map[string]any{"protocol_versions": []string{LegacyVersion, ModernVersion}, "scoped_existing_reads": true, "managed_registration_sessions": true, "backend_session_proof": false, "unmanaged_sessions": false, "remote_reads": false, "output_preview": false, "process_create": s.creator != nil, "human_handoff": map[string]string{"stop_agent": "txm agent stop FULL_MANAGED_ID --confirm FULL_MANAGED_ID", "cleanup_task": "txm task cleanup FULL_TASK_ID --dry-run", "merge_task": "txm task merge FULL_TASK_ID --target refs/heads/BRANCH"}, "destructive_tools": false, "max_message_bytes": MaxMessage, "max_requests_per_process": MaxRequests, "max_concurrent_requests": 16, "max_concurrent_waits": 4}}, nil
	})
	return s, nil
}

// EnableCreates is called only during startup after loading an operator-owned
// policy. No tool can add templates or change this authorization.
func (s *Server) EnableCreates(c *control.Creator) {
	s.creator = c
	for _, kind := range []string{"agent", "task"} {
		if !c.Allows(kind) {
			continue
		}
		kind := kind
		yes := true
		input := schema(map[string]any{"operation_id": map[string]any{"type": "string", "pattern": "^[a-f0-9]{32}$"}, "template_id": map[string]any{"type": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$"}}, []string{"operation_id", "template_id"})
		sdk.AddTool(s.sdk, &sdk.Tool{Name: "start_" + kind, Description: "Create only from an operator-approved startup template. Permanent operation IDs prevent replay; uncertain results require inspection, never a new retry ID. Child environment is empty.", InputSchema: input, Annotations: &sdk.ToolAnnotations{DestructiveHint: &yes, IdempotentHint: true, OpenWorldHint: &yes}}, func(ctx context.Context, _ *sdk.CallToolRequest, in control.CreateInput) (*sdk.CallToolResult, Output, error) {
			bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			var v control.Creation
			var err error
			select {
			case s.requests <- struct{}{}:
				defer func() { <-s.requests }()
				v, err = c.Create(bounded, kind, in)
			default:
				err = core.Failure("conflict", "MCP request capacity reached")
			}
			out := Output{SchemaVersion: 1, Data: v}
			if err != nil {
				out.Data = nil
				out.Error = core.Normalize(err)
			}
			return &sdk.CallToolResult{IsError: err != nil, Content: []sdk.Content{&sdk.TextContent{Text: "Inspect structuredContent; uncertain reservations must not be relaunched."}}}, out, nil
		})
	}
}

func schema(properties map[string]any, required []string) map[string]any {
	s := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func addRead[In any](s *Server, name, description string, input any, wait bool, call func(context.Context, In) (Output, error)) {
	no := false
	sdk.AddTool(s.sdk, &sdk.Tool{Name: name, Description: description, InputSchema: input, Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &no, IdempotentHint: true, OpenWorldHint: &no}}, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, Output, error) {
		var result Output
		var err error
		select {
		case s.requests <- struct{}{}:
			defer func() { <-s.requests }()
		default:
			err = core.Failure("conflict", "MCP request capacity reached")
		}
		if err == nil && wait {
			select {
			case s.waits <- struct{}{}:
				defer func() { <-s.waits }()
			default:
				err = core.Failure("conflict", "MCP wait capacity reached")
			}
		}
		if err == nil {
			budget := core.RequestTimeout
			if wait {
				budget = time.Minute
			}
			bounded, cancel := context.WithTimeout(ctx, budget)
			defer cancel()
			result, err = call(bounded, in)
		}
		result.SchemaVersion = 1
		message := "Scoped result is in structuredContent."
		if err != nil {
			result.Data = nil
			result.Error = core.Normalize(err)
			message = result.Error.Code + ": " + result.Error.Message
		}
		return &sdk.CallToolResult{IsError: err != nil, Content: []sdk.Content{&sdk.TextContent{Text: message}}}, result, nil
	})
}

type cursor struct {
	Tool      string `json:"tool"`
	Inventory string `json:"inventory"`
	Offset    int    `json:"offset"`
}

func page[T any](s *Server, name string, in ListInput, items []T, id func(T) string) (Output, error) {
	if in.Limit == 0 {
		in.Limit = 128
	}
	if in.Limit < 1 || in.Limit > 256 {
		return Output{}, core.Failure("invalid_usage", "page limit must be 1..256")
	}
	ids := make([]string, len(items))
	for i, v := range items {
		ids[i] = id(v)
	}
	digest := sha256.Sum256([]byte(strings.Join(ids, "\n")))
	inventory := hex.EncodeToString(digest[:])
	start := 0
	if in.Cursor != "" {
		parts := strings.Split(in.Cursor, ".")
		if len(parts) != 2 || len(in.Cursor) > 512 {
			return Output{}, core.Failure("invalid_usage", "invalid scoped cursor")
		}
		body, e := base64.RawURLEncoding.DecodeString(parts[0])
		sig, se := base64.RawURLEncoding.DecodeString(parts[1])
		mac := hmac.New(sha256.New, s.key[:])
		_, _ = mac.Write(body)
		var c cursor
		if e != nil || se != nil || !hmac.Equal(sig, mac.Sum(nil)) || nativeprotocol.Decode(body, &c) != nil || c.Tool != name || c.Inventory != inventory || c.Offset < 0 || c.Offset > len(items) {
			return Output{}, core.Failure("conflict", "cursor inventory or read scope changed")
		}
		start = c.Offset
	}
	end, size := start, 2048
	for end < len(items) && end-start < in.Limit {
		b, e := json.Marshal(items[end])
		if e != nil {
			return Output{}, e
		}
		if size+len(b) > 512<<10 {
			break
		}
		size += len(b) + 1
		end++
	}
	if end == start && start < len(items) {
		return Output{}, core.Failure("protocol_error", "scoped item exceeds output budget")
	}
	out := Output{Data: map[string]any{"items": items[start:end]}, Partial: end < len(items)}
	if out.Partial {
		body, _ := json.Marshal(cursor{name, inventory, end})
		mac := hmac.New(sha256.New, s.key[:])
		_, _ = mac.Write(body)
		out.NextCursor = base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	return out, nil
}

func eraMiddleware() sdk.Middleware {
	var mu sync.Mutex
	era := ""
	initialized := false
	return func(next sdk.MethodHandler) sdk.MethodHandler {
		return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
			var meta map[string]any
			params := req.GetParams()
			if params != nil && (reflect.ValueOf(params).Kind() != reflect.Pointer || !reflect.ValueOf(params).IsNil()) {
				meta = params.GetMeta()
			}
			version, _ := meta[sdk.MetaKeyProtocolVersion].(string)
			if method == "initialize" {
				p, ok := params.(*sdk.InitializeParams)
				if !ok || p == nil || p.ProtocolVersion == "" || p.Capabilities == nil || p.ClientInfo == nil {
					return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "initialize requires protocolVersion, capabilities and clientInfo"}
				}
			}
			if version != "" && version != ModernVersion {
				data, _ := json.Marshal(sdk.UnsupportedProtocolVersionData{Requested: version, Supported: []string{LegacyVersion, ModernVersion}})
				return nil, &jsonrpc.Error{Code: sdk.CodeUnsupportedProtocolVersion, Message: "unsupported protocol version", Data: data}
			}
			mu.Lock()
			bad := func() (sdk.Result, error) {
				mu.Unlock()
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidRequest, Message: "MCP protocol eras must not be mixed"}
			}
			if method == "initialize" {
				p, ok := params.(*sdk.InitializeParams)
				if !ok || p == nil || version != "" || era == "modern" {
					return bad()
				}
				// Legacy negotiation offers the supported legacy version even when
				// the client proposed an unsupported one. It never initializes modern MCP.
				p.ProtocolVersion = LegacyVersion
				era = "legacy"
			} else if method == "notifications/initialized" {
				if era != "legacy" || version != "" {
					return bad()
				}
				initialized = true
			} else if method == "ping" && version == "" && era != "modern" {
				// Legacy ping is allowed during initialization.
			} else if method != "notifications/cancelled" {
				if version == ModernVersion {
					if era == "legacy" {
						return bad()
					}
					era = "modern"
				} else if era != "legacy" || !initialized || version != "" {
					return bad()
				}
			}
			mu.Unlock()
			return next(ctx, method, req)
		}
	}
}

func (s *Server) Run(ctx context.Context, input io.ReadCloser, output io.WriteCloser) error {
	bounded, cancel := context.WithCancel(ctx)
	defer cancel()
	t := transport(input, output)
	reader := t.Reader.(*frameReader)
	reader.onEOF = cancel
	err := s.sdk.Run(bounded, t)
	reader.flow.mu.Lock()
	cleanEOF := reader.flow.eof
	reader.flow.mu.Unlock()
	if ctx.Err() == nil && cleanEOF {
		return nil
	}
	return err
}
