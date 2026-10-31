//go:build linux || darwin

package agent

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/localipc"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

type coordinator struct {
	state    *runtimepath.Root
	registry Registry
	gate     chan struct{}
	epoch    string
	hub      *eventHub
	wake     chan struct{}
}

func Serve(ctx context.Context, epoch string, ready func(error)) (retErr error) {
	if !supervisor.Supported() {
		return core.Failure("unsupported", "agent coordinator requires Unix and CGO")
	}
	if !runtimepath.ValidID(epoch) {
		return core.Failure("invalid_usage", "invalid coordinator epoch")
	}
	sent := false
	notify := func(e error) {
		if !sent {
			sent = true
			if ready != nil {
				ready(e)
			}
		}
	}
	defer func() { notify(retErr) }()
	runtime, e := supervisor.Runtime()
	if e != nil {
		return e
	}
	defer func() { _ = runtime.Close() }()
	lock, e := runtime.Lock("agent-coordinator")
	if e != nil {
		return core.Failure("conflict", "coordinator already active or lock unsafe")
	}
	defer func() { _ = lock.Close() }()
	state, e := supervisor.State()
	if e != nil {
		return e
	}
	defer func() { _ = state.Close() }()
	stateLock, e := state.Lock("agent-registry")
	if e != nil {
		return core.Failure("conflict", "another coordinator owns the state registry")
	}
	defer func() { _ = stateLock.Close() }()
	host, e := runtimepath.HostID(ctx)
	if e != nil {
		return e
	}
	registry, e := LoadRegistry(state, host)
	if e != nil {
		return e
	}
	for _, r := range registry.Agents {
		if r.Supervised {
			discardUnusedManifest(state, *r.InstanceID)
		}
	}
	server := &coordinator{state: state, registry: registry, gate: make(chan struct{}, 1), epoch: epoch, hub: newHub(epoch), wake: make(chan struct{}, 1)}
	path, e := runtime.SocketPath(epoch)
	if e != nil {
		return e
	}
	listener, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		return e
	}
	listener.SetUnlinkOnClose(false)
	defer func() { _ = listener.Close() }()
	if e = os.Chmod(path, 0600); e != nil {
		return e
	}
	original, e := runtime.SocketInfo(epoch)
	if e != nil {
		return e
	}
	defer func() { _ = runtime.RemoveSocket(epoch, original) }()
	metadata, _ := json.Marshal(Discovery{1, epoch})
	if e = runtime.WritePrivate("coordinator.json", metadata, true); e != nil {
		return e
	}
	// Hold both locks until workers finish. An old epoch never removes a new
	// coordinator's discovery file; crash recovery leaves stale sockets intact.
	defer func() {
		data, e := runtime.ReadPrivate("coordinator.json", 256)
		if e == nil && string(data) == string(metadata) {
			_ = runtime.RemovePrivate("coordinator.json")
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	clients := make(chan struct{}, 16)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	workers.Add(1)
	go func() { defer workers.Done(); server.watch(ctx) }()
	notify(nil)
	for {
		c, e := listener.Accept()
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		select {
		case clients <- struct{}{}:
		default:
			_ = c.Close()
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			holdsSlot := true
			defer func() {
				if holdsSlot {
					<-clients
				}
				_ = c.Close()
			}()
			requestCtx, done := Context(ctx)
			defer done()
			closeOnCancel := context.AfterFunc(requestCtx, func() { _ = c.Close() })
			defer closeOnCancel()
			deadline, _ := requestCtx.Deadline()
			_ = c.SetDeadline(deadline)
			if runtimepath.VerifyPeer(c) != nil {
				return
			}
			var q Request
			reply := Reply{SchemaVersion: 1, Epoch: epoch, Agents: []View{}}
			if e := localipc.Read(c, &q); e != nil {
				reply.Error = core.Normalize(e)
			} else if q.SchemaVersion != 1 || q.Epoch != epoch {
				reply.Error = core.Failure("unsupported_version", "coordinator schema/epoch mismatch")
			} else if q.HostID != server.registry.HostID || q.StateRoot != server.state.Path {
				reply.Error = core.Failure("conflict", "coordinator belongs to a different state identity")
			} else {
				if q.Op == "subscribe" {
					closeOnCancel()
					done()
					_ = c.SetDeadline(time.Time{})
					holdsSlot = false
					<-clients
					server.stream(ctx, c, q)
					return
				}
				select {
				case server.gate <- struct{}{}:
					reply.Agents, e = server.handle(requestCtx, q)
					if e != nil {
						reply.Error = core.Normalize(e)
					}
					<-server.gate
				case <-requestCtx.Done():
					reply.Error = core.Normalize(requestCtx.Err())
				}
			}
			if reply.Agents == nil {
				reply.Agents = []View{}
			}
			_ = localipc.Write(c, reply)
			if q.Op == "stop" && reply.Error == nil {
				cancel()
			}
		}()
	}
}
func (s *coordinator) observe(ctx context.Context, r *Record) (v View, retErr error) {
	v = View{ID: r.ID, ManagedID: r.ManagedID, InstanceID: r.InstanceID, Locator: r.Locator, Supervised: r.Supervised, Lifecycle: "unknown", Attention: "unknown", AttentionSource: "no verified hooks", Source: "unverified backend binding", ObservedAt: supervisor.Now()}
	defer func() {
		if retErr == nil && ctx.Err() == nil {
			projected, err := projectExternalHook(s.state, *r)
			if err != nil {
				retErr = err
				return
			}
			v = hookView(v, projected)
			if s.hub != nil {
				s.hub.track(v)
			}
		}
	}()
	if r.Supervised {
		receipt, e := supervisor.ReadReceipt(s.state, *r.ManagedID, *r.InstanceID, r.Locator.Backend)
		if e == nil {
			v.Lifecycle = "exited"
			v.Source = "supervisor receipt"
			v.ObservedAt = receipt.FinishedAt
			v.Receipt = receipt
			return v, nil
		}
		if !os.IsNotExist(e) {
			return v, e
		}
	}
	b, e := backend.NewBackend(r.Locator.Backend, ctx)
	if e != nil {
		return v, nil
	}
	observed, e := backend.ObserveBinding(ctx, b, r.Binding)
	if e == nil {
		r.Locator.Name = observed.Locator.Name
		v.Locator = r.Locator
		v.Source = "backend observation (unsupervised)"
	}
	if !r.Supervised {
		return v, nil
	}
	// A wrapper handshake proves the child, not a reused backend ID or PID.
	status, e := supervisor.Probe(ctx, *r.ManagedID, *r.InstanceID)
	if e == nil && status.Running {
		v.Lifecycle = "running"
		v.Source = "supervisor kernel observation"
		v.ObservedAt = status.ObservedAt
		return v, nil
	}
	// Close the race in which receipt publication followed the initial read.
	receipt, e := supervisor.ReadReceipt(s.state, *r.ManagedID, *r.InstanceID, r.Locator.Backend)
	if e == nil {
		v.Lifecycle = "exited"
		v.Source = "supervisor receipt"
		v.ObservedAt = receipt.FinishedAt
		v.Receipt = receipt
		return v, nil
	}
	if !os.IsNotExist(e) {
		return v, e
	}
	v.Source = "supervisor unavailable; no receipt"
	return v, nil
}
func (s *coordinator) handle(ctx context.Context, q Request) ([]View, error) {
	if e := q.validate(); e != nil {
		return nil, e
	}
	if q.Op == "start-metrics" {
		q.Op = "start"
	}
	if q.Op == "stop" || q.Op == "ping" {
		return []View{}, nil
	}
	if q.Op == "prune" {
		return s.prune(ctx, q)
	}
	if q.Op == "terminate" {
		return s.terminate(ctx, q.ID)
	}
	if q.Op == "start" && q.ID != "" {
		for _, r := range s.registry.Agents {
			if r.ID == q.ID {
				return nil, core.Failure("conflict", "managed ID already used; launch will not be replayed")
			}
		}
	}
	if q.Op == "start" && q.Provider == "claude" {
		if err := ValidateAutomaticClaudeArgs(q.Argv); err != nil {
			return nil, err
		}
	}
	if q.Op == "hooks-enable" || q.Op == "hooks-info" || q.Op == "signal" || q.Op == "provider-event" {
		return s.hookRequest(ctx, q)
	}
	if q.Op != "start" && q.Op != "register" && q.Op != "list" && q.Op != "status" && q.Op != "refresh" {
		return nil, core.Failure("invalid_usage", "unknown agent request")
	}
	if q.Op == "status" && !runtimepath.ValidID(q.ID) {
		return nil, core.Failure("invalid_target", "agent status requires a full agent ID")
	}
	views := []View{}
	found := false
	for i := range s.registry.Agents {
		r := &s.registry.Agents[i]
		if (q.Op == "list" || q.Op == "status") && q.Backend != "" && r.Locator.Backend != q.Backend {
			if q.Op == "status" && r.ID == q.ID {
				return nil, core.Failure("invalid_target", "agent backend conflicts with --backend")
			}
			continue
		}
		if q.Op == "status" && r.ID != q.ID {
			continue
		}
		v, e := s.observe(ctx, r)
		if e != nil {
			return nil, e
		}
		views = append(views, v)
		found = true
	}
	if q.Op == "status" && !found {
		return nil, core.Failure("not_found", "agent ID not registered")
	}
	if q.Op == "list" || q.Op == "status" || q.Op == "refresh" {
		if e := saveRegistry(s.state, s.registry); e != nil {
			return nil, e
		}
		sortedViews(views)
		return views, nil
	}
	if len(s.registry.Agents) >= 256 {
		return nil, core.Failure("conflict", "agent registry limit reached")
	}
	if q.Name == "" || len(q.Name) > 512 || (q.Op == "start" && runtimepath.ValidateName(q.Name) != nil) {
		return nil, core.Failure("invalid_usage", "invalid agent name")
	}
	for _, v := range views {
		if v.Locator.Backend == q.Backend && v.Locator.Name == q.Name && v.Lifecycle != "exited" {
			return nil, core.Failure("conflict", "agent name is already registered")
		}
	}
	b, e := backend.NewBackend(q.Backend, ctx)
	if e != nil {
		return nil, e
	}
	if e = backend.PreflightManaged(ctx, b); e != nil {
		return nil, e
	}
	id, e := runtimepath.NewID()
	if e != nil {
		return nil, e
	}
	if q.ID != "" {
		id = q.ID
	}
	history, err := s.forgotten()
	if err != nil {
		return nil, err
	}
	for _, used := range history.IDs {
		if used == id {
			return nil, core.Failure("conflict", "managed ID was pruned; launch will not be replayed")
		}
	}
	r := Record{ID: id, Locator: core.SessionLocator{HostID: s.registry.HostID, Backend: q.Backend, Name: q.Name}, CreatedAt: supervisor.Now()}
	if q.Op == "register" {
		observed, e := b.Lookup(ctx, q.Name)
		if e != nil {
			return nil, e
		}
		if observed.BackendID != nil {
			r.Binding.BackendID = *observed.BackendID
		}
		if observed.InstanceID != nil {
			r.Binding.NativeInstance = *observed.InstanceID
		}
		r.Binding, e = backend.RegisterBinding(ctx, b, observed, id)
		if e != nil {
			return nil, e
		}
		if e = r.validate(s.registry.HostID); e != nil {
			return nil, e
		}
		s.registry.Agents = append(s.registry.Agents, r)
		if e = saveRegistry(s.state, s.registry); e != nil {
			s.registry.Agents = s.registry.Agents[:len(s.registry.Agents)-1]
			return nil, e
		}
		v, e := s.observe(ctx, &s.registry.Agents[len(s.registry.Agents)-1])
		return []View{v}, e
	}
	instance, e := runtimepath.NewID()
	if e != nil {
		return nil, e
	}
	m, e := CompileStart(q.Name, q.Backend, q.Cwd, q.Argv, q.EnvNames)
	if e != nil {
		return nil, e
	}
	m.ManagedID = id
	m.InstanceID = instance
	m.Receipt = supervisor.ReceiptName(instance)
	m.Metrics = q.Metrics
	if _, e = b.Lookup(ctx, q.Name); e == nil {
		return nil, core.Failure("conflict", "session already exists")
	} else if core.Normalize(e).Code != "not_found" {
		return nil, e
	}
	r.Supervised = true
	r.ManagedID = &id
	r.InstanceID = &instance
	var hookCredential *Credential
	cleanupHooks := func() {
		if hookCredential != nil {
			_ = s.state.RemovePrivate(credentialName(hookCredential.Generation))
			_ = s.state.RemovePrivate(externalCredentialName(hookCredential.Generation))
			_ = s.state.RemovePrivate("p-" + hookCredential.Generation + ".json")
			if !legacyProvider(hookCredential.Provider, hookCredential.Version) {
				_ = s.state.RemovePrivate(externalAssociationName(id))
			}
		}
	}
	if q.Provider != "" {
		if q.Provider != "generic" && q.Provider != "claude" {
			return nil, core.Failure("unsupported", "automatic provider configuration unavailable; use reviewed hooks configuration")
		}
		var c Credential
		var err error
		if legacyProvider(q.Provider, q.ProviderVersion) {
			c, err = s.enableHook(&r, q.Provider, q.ProviderVersion)
		} else {
			c, err = s.enableExternalHook(&r, q.Provider, q.ProviderVersion)
		}
		if err != nil {
			return nil, err
		}
		hookCredential = &c
		m.Hooks = &supervisor.HookContext{AgentID: id, InstanceID: instance, Generation: c.Generation, StateHome: c.Namespace.StateHome, RuntimeDir: c.Namespace.RuntimeDir, TempDir: c.Namespace.TempDir}
		if q.Provider == "claude" {
			exe, err := os.Executable()
			if err != nil {
				cleanupHooks()
				return nil, err
			}
			config, err := ProviderConfig(exe, c)
			if err != nil {
				cleanupHooks()
				return nil, err
			}
			configName := "p-" + c.Generation + ".json"
			if err = s.state.WritePrivate(configName, config, false); err != nil {
				cleanupHooks()
				return nil, err
			}
			m.Argv = append([]string{m.Argv[0], "--settings", filepath.Join(s.state.Path, configName)}, m.Argv[1:]...)
		}
	}
	if e = supervisor.WriteManifest(s.state, m); e != nil {
		cleanupHooks()
		return nil, e
	}
	// Persist launch intent before any backend effect. An interrupted coordinator
	// may leave an unknown record; restart never consumes/replays its manifest.
	s.registry.Agents = append(s.registry.Agents, r)
	if e = saveRegistry(s.state, s.registry); e != nil {
		s.registry.Agents = s.registry.Agents[:len(s.registry.Agents)-1]
		_ = s.state.RemovePrivate(supervisor.ManifestName(instance))
		cleanupHooks()
		return nil, e
	}
	executable, e := os.Executable()
	if e != nil {
		return nil, e
	}
	executable, e = filepath.EvalSymlinks(executable)
	if e != nil {
		return nil, e
	}
	binding, launchErr := backend.LaunchManaged(ctx, b, q.Name, executable, instance, m.Cwd)
	s.registry.Agents[len(s.registry.Agents)-1].Binding = binding
	if e = saveRegistry(s.state, s.registry); e != nil {
		return nil, core.Failure("partial_failure", "managed launch state uncertain; inspect agent "+id)
	}
	// Wait briefly for a live child or receipt, including native readiness that
	// fails because a correctly supervised short-lived child already exited.
	var v View
	for tries := 0; tries < 40; tries++ {
		v, e = s.observe(ctx, &s.registry.Agents[len(s.registry.Agents)-1])
		if e != nil {
			return []View{v}, e
		}
		if v.Receipt != nil || v.Lifecycle == "running" {
			break
		}
		select {
		case <-ctx.Done():
			return []View{v}, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	if v.Receipt != nil {
		if v.Receipt.Outcome == "start_failed" {
			return []View{v}, core.Failure("operation_failed", "managed executable failed to start; receipt recorded")
		}
		return []View{v}, nil
	}
	if v.Lifecycle == "running" {
		return []View{v}, nil
	}
	if launchErr != nil {
		discardUnusedManifest(s.state, instance)
	}
	return []View{v}, core.Failure("partial_failure", "managed launch unproven; inspect agent "+id+"; no automatic retry")
}

func discardUnusedManifest(state *runtimepath.Root, id string) {
	lease, e := state.Lock("supervisor-" + id)
	if e != nil {
		return
	}
	defer func() { _ = lease.Close() }()
	// A free lease plus a one-use private file proves no wrapper has consumed
	// it. Never replay an interrupted launch on coordinator restart.
	_ = state.RemovePrivate(supervisor.ManifestName(id))
}
