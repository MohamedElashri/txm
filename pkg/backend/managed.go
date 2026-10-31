package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/profile"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

// ManagedBinding is private registry metadata, never a PID-based identity.
type ManagedBinding struct {
	BackendID      string `json:"backend_id"`
	NativeInstance string `json:"native_instance"`
	Owner          string `json:"owner"`
}

func PreflightManaged(ctx context.Context, b TerminalMultiplexer) error {
	if !supervisor.Supported() {
		return unsupported(b.Name(), "managed launch")
	}
	if b.Name() != "native" && b.Name() != "tmux" {
		return unsupported(b.Name(), "managed launch/registration")
	}
	if !b.IsAvailable() {
		return core.Failure("backend_unavailable", "managed backend unavailable")
	}
	if b.Name() == "tmux" {
		return PreflightProfile(ctx, b)
	}
	return nil
}
func LaunchManaged(ctx context.Context, b TerminalMultiplexer, name, executable, id, cwd string) (ManagedBinding, error) {
	var binding ManagedBinding
	if e := PreflightManaged(ctx, b); e != nil {
		return binding, e
	}
	if runtimepath.ValidateName(name) != nil || !runtimepath.ValidID(id) || !filepath.IsAbs(executable) || !filepath.IsAbs(cwd) {
		return binding, core.Failure("invalid_usage", "invalid managed launcher target")
	}
	if _, e := b.Lookup(ctx, name); e == nil {
		return binding, core.Failure("conflict", "session already exists")
	} else if core.Normalize(e).Code != "not_found" {
		return binding, e
	}
	switch adapter := b.(type) {
	case *NativeBackend:
		instance, e := runtimepath.NewID()
		if e != nil {
			return binding, e
		}
		binding = ManagedBinding{BackendID: instance, NativeInstance: instance}
		return binding, startNativeInDir(ctx, name, instance, []string{executable, "internal", "supervised-run", id}, cwd)
	case *TmuxBackend:
		token, e := runtimepath.NewID()
		if e != nil {
			return binding, e
		}
		binding.Owner = token
		args := []string{"new-session", "-d", "-P", "-F", "#{session_id}|#{window_id}|#{pane_id}", "-s", name, "-c", cliArgument(literalDir(cwd)), "-e", "TXM_MANAGED_INSTANCE=" + token}
		// These select private txm namespaces only. User argv/env values never
		// enter launch command text; the child allowlist is applied by wrapper.
		for _, key := range []string{"HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "TMPDIR"} {
			args = append(args, "-e", cliArgument(key+"="+os.Getenv(key)))
		}
		args = append(args, "--", cliArgument(executable), "internal", "supervised-run", id)
		raw, diag, _, e := adapter.output(ctx, 4096, false, args...)
		if e != nil {
			if len(diag) >= 18 && diag[:18] == "duplicate session:" {
				return binding, core.Failure("conflict", "session already exists")
			}
			return binding, e
		}
		created, e := receipt(raw)
		if e != nil {
			return binding, e
		}
		binding.BackendID = created.SessionID
		return binding, nil
	}
	return binding, unsupported(b.Name(), "managed launch")
}
func ObserveBinding(ctx context.Context, b TerminalMultiplexer, binding ManagedBinding) (core.Session, error) {
	list, e := b.Sessions(ctx)
	if e != nil {
		return core.Session{}, e
	}
	for _, s := range list {
		match := s.BackendID != nil && *s.BackendID == binding.BackendID
		if b.Name() == "native" {
			match = s.InstanceID != nil && *s.InstanceID == binding.NativeInstance
		}
		if !match {
			continue
		}
		if adapter, ok := b.(*TmuxBackend); ok && binding.Owner != "" {
			out, _, _, e := adapter.output(ctx, 128, false, "display-message", "-p", "-t", binding.BackendID, "#{TXM_MANAGED_INSTANCE}")
			if e != nil {
				return core.Session{}, e
			}
			if out != binding.Owner+"\n" {
				return core.Session{}, core.Failure("conflict", "managed backend ownership changed")
			}
		}
		return s, nil
	}
	return core.Session{}, core.Failure("not_found", "bound session no longer exists")
}

func RegisterBinding(ctx context.Context, b TerminalMultiplexer, s core.Session, id string) (ManagedBinding, error) {
	var binding ManagedBinding
	if !runtimepath.ValidID(id) || s.BackendID == nil {
		return binding, core.Failure("invalid_target", "registration requires an exact backend identity")
	}
	binding.BackendID = *s.BackendID
	if b.Name() == "native" {
		if s.InstanceID == nil || !runtimepath.ValidID(*s.InstanceID) {
			return binding, core.Failure("invalid_target", "native registration requires an instance")
		}
		binding.NativeInstance = *s.InstanceID
		return binding, nil
	}
	adapter, ok := b.(*TmuxBackend)
	if !ok {
		return binding, unsupported(b.Name(), "registration")
	}
	if !tmuxSessionID.MatchString(binding.BackendID) {
		return binding, core.Failure("protocol_error", "invalid tmux registration identity")
	}
	target := binding.BackendID + ":"
	raw, _, _, e := adapter.output(ctx, 128, false, "display-message", "-p", "-t", target, "#{TXM_MANAGED_INSTANCE}")
	if e != nil {
		return binding, e
	}
	owner := strings.TrimSpace(raw)
	if owner == "" {
		condition := "#{&&:#{==:#{session_id}," + binding.BackendID + "},#{==:#{TXM_MANAGED_INSTANCE},}}"
		_, _, _, e = adapter.output(ctx, 128, false, "if-shell", "-F", "-t", target, condition, tmuxCommand("set-environment", "-t", binding.BackendID, "TXM_MANAGED_INSTANCE", id), "show-options -t "+binding.BackendID+" __txm_registration_changed")
		if e != nil {
			return binding, e
		}
		owner = id
	}
	if !runtimepath.ValidID(owner) {
		return binding, core.Failure("conflict", "session binding metadata is invalid")
	}
	binding.Owner = owner
	return binding, nil
}

// TerminateManaged verifies ownership immediately at the destructive backend boundary.
func TerminateManaged(ctx context.Context, b TerminalMultiplexer, binding ManagedBinding) error {
	if b.Name() == "native" && (!runtimepath.ValidID(binding.NativeInstance) || binding.BackendID != binding.NativeInstance) {
		return core.Failure("conflict", "invalid native binding")
	}
	if b.Name() == "tmux" && (!runtimepath.ValidID(binding.Owner) || !tmuxSessionID.MatchString(binding.BackendID)) {
		return core.Failure("conflict", "invalid tmux binding")
	}
	s, e := ObserveBinding(ctx, b, binding)
	if e != nil {
		return e
	}
	switch a := b.(type) {
	case *NativeBackend:
		c, e := a.profileClient(ctx, profile.Applied{Name: s.Locator.Name, InstanceID: binding.NativeInstance}, "control")
		if e != nil {
			return e
		}
		defer func() { _ = c.Close() }()
		return a.killClient(c)
	case *TmuxBackend:
		condition := "#{&&:#{==:#{session_id}," + binding.BackendID + "},#{==:#{TXM_MANAGED_INSTANCE}," + binding.Owner + "}}"
		_, _, _, e := a.output(ctx, 128, false, "if-shell", "-F", "-t", binding.BackendID+":", condition, tmuxCommand("kill-session", "-t", binding.BackendID), "show-options -t "+binding.BackendID+" __txm_managed_owner_mismatch")
		return e
	}
	return unsupported(b.Name(), "managed termination")
}
