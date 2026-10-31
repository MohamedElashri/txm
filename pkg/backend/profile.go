package backend

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/profile"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

// ApplyProfile accepts a preflighted plan; only these adapters have exact apply
// and ownership semantics. External command effects cannot be rolled back.
func ApplyProfile(ctx context.Context, b TerminalMultiplexer, plan profile.Plan) (profile.Applied, error) {
	if plan.Backend != b.Name() || len(plan.Windows) == 0 {
		return profile.Applied{}, core.Failure("invalid_profile", "profile plan/backend mismatch")
	}
	if e := PreflightProfile(ctx, b); e != nil {
		return profile.Applied{}, e
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	switch backend := b.(type) {
	case *TmuxBackend:
		return backend.applyProfile(ctx, plan)
	case *NativeBackend:
		return backend.applyProfile(ctx, plan)
	}
	return profile.Applied{}, unsupported(b.Name(), "profile application")
}

// PreflightProfile probes no session state and never initializes native paths.
func PreflightProfile(ctx context.Context, b TerminalMultiplexer) error {
	caps, e := b.BackendCapabilities(ctx)
	if e != nil {
		return e
	}
	if b.Name() == "tmux" {
		if caps.Version == nil {
			return core.Failure("unsupported_version", "tmux profile version is unknown")
		}
		v := strings.TrimPrefix(*caps.Version, "tmux 3.")
		v = strings.TrimRight(v, "abcdefghijklmnopqrstuvwxyz")
		minor, e := strconv.Atoi(v)
		if e != nil || minor < 4 {
			return core.Failure("unsupported_version", "profiles require tmux 3.4 or newer")
		}
	}
	return nil
}
func AttachProfile(ctx context.Context, b TerminalMultiplexer, owned profile.Applied, readOnly bool) error {
	switch backend := b.(type) {
	case *TmuxBackend:
		return backend.attachProfile(ctx, owned, readOnly)
	case *NativeBackend:
		return backend.attachProfile(ctx, owned, readOnly)
	}
	return unsupported(b.Name(), "profile attach")
}
func profileFailure(cause error, owned profile.Applied, clean bool) error {
	e := core.Normalize(cause)
	code := e.Code
	message := "profile apply failed; no owned session survives; accepted input may have run"
	if !clean {
		code = "partial_failure"
		message = fmt.Sprintf("partial profile apply; resources may survive at %s/%s (ID %s); ownership uncertain, no deletion by name", owned.Backend, owned.Name, owned.SessionID)
		if errors.Is(cause, context.Canceled) || e.Code == "cancelled" {
			code = "cancelled"
		}
	}
	result := core.Scope(core.Failure(code, message), owned.Backend)
	result.Cause = cause
	return result
}
func quoteTmux(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func tmuxCommand(args ...string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = quoteTmux(arg)
	}
	return strings.Join(quoted, " ")
}
func literalDir(dir string) string { return strings.ReplaceAll(dir, "#", "##") }
func cliArgument(arg string) string {
	if strings.HasSuffix(arg, ";") {
		return arg[:len(arg)-1] + "\\;"
	}
	return arg
}
func receipt(raw string) (profile.Applied, error) {
	f := strings.Split(strings.TrimSpace(raw), "|")
	if len(f) != 3 || !tmuxSessionID.MatchString(f[0]) || !tmuxWindowID.MatchString(f[1]) || !tmuxPaneID.MatchString(f[2]) {
		return profile.Applied{}, core.Failure("protocol_error", "invalid profile creation receipt")
	}
	return profile.Applied{Backend: "tmux", SessionID: f[0], Windows: []profile.CreatedWindow{{ID: f[1], Panes: []string{f[2]}}}}, nil
}

// The guard and selected command execute on one server queue. A server restart
// drops that queue rather than reusing an ID on a different server. Tokens also
// refuse replacement sessions; target context checks prevent focus/ID guessing.
func (b *TmuxBackend) profileGuard(ctx context.Context, owned profile.Applied, window, pane string, args ...string) (string, error) {
	if !tmuxSessionID.MatchString(owned.SessionID) || !runtimepath.ValidID(owned.Owner) {
		return "", core.Failure("conflict", "missing profile ownership receipt")
	}
	target := owned.SessionID + ":"
	condition := "#{&&:#{==:#{TXM_PROFILE_OWNER}," + owned.Owner + "},#{==:#{session_id}," + owned.SessionID + "}}"
	if window != "" {
		if !tmuxWindowID.MatchString(window) {
			return "", core.Failure("protocol_error", "invalid profile window receipt")
		}
		target += window
		condition = "#{&&:" + condition + ",#{==:#{window_id}," + window + "}}"
		condition = "#{&&:" + condition + ",#{==:#{window_linked},0}}"
	}
	if pane != "" {
		if !tmuxPaneID.MatchString(pane) {
			return "", core.Failure("protocol_error", "invalid profile pane receipt")
		}
		target += "." + pane
		condition = "#{&&:" + condition + ",#{==:#{pane_id}," + pane + "}}"
	}
	if len(args) > 0 && args[0] == "send-keys" {
		condition = "#{&&:" + condition + ",#{==:#{synchronize-panes},0}}"
	}
	refusal := "show-options -t " + owned.SessionID + " __txm_profile_owner_mismatch"
	out, _, _, e := b.output(ctx, 4096, false, "if-shell", "-F", "-t", target, condition, tmuxCommand(args...), refusal)
	return out, e
}
func (b *TmuxBackend) rollbackProfile(owned profile.Applied) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sessions, e := b.Sessions(ctx)
	if e != nil {
		return false
	}
	exists := false
	for _, s := range sessions {
		if s.BackendID != nil && *s.BackendID == owned.SessionID {
			exists = true
		}
	}
	if !exists {
		return true
	}
	if _, e = b.profileGuard(ctx, owned, "", "", "kill-session", "-t", owned.SessionID); e != nil {
		return false
	}
	sessions, e = b.Sessions(ctx)
	if e != nil {
		return false
	}
	for _, s := range sessions {
		if s.BackendID != nil && *s.BackendID == owned.SessionID {
			return false
		}
	}
	return true
}
func (b *TmuxBackend) applyProfile(ctx context.Context, plan profile.Plan) (owned profile.Applied, retErr error) {
	owned = profile.Applied{Backend: "tmux", Name: plan.Name}
	if _, e := b.probeVersion(ctx); e != nil {
		return owned, e
	}
	if e := ctx.Err(); e != nil {
		return owned, e
	}
	token, e := runtimepath.NewID()
	if e != nil {
		return owned, e
	}
	initial := plan.Windows[0]
	args := []string{"new-session", "-d", "-P", "-F", "#{session_id}|#{window_id}|#{pane_id}", "-s", plan.Name, "-n", initial.Name, "-c", cliArgument(literalDir(initial.Panes[0].Dir)), "-e", "TXM_PROFILE_OWNER=" + token}
	args = append(args, "--", cliArgument(plan.Shell), "-i")
	raw, diag, _, e := b.output(ctx, 4096, false, args...)
	if e != nil {
		if strings.HasPrefix(strings.TrimSpace(diag), "duplicate session:") {
			return owned, core.Failure("conflict", "profile session already exists")
		}
		return owned, profileFailure(e, owned, false)
	}
	created, e := receipt(raw)
	if e != nil {
		return owned, profileFailure(e, owned, false)
	}
	owned = created
	owned.Name = plan.Name
	owned.Owner = token
	defer func() {
		if retErr != nil {
			retErr = profileFailure(retErr, owned, b.rollbackProfile(owned))
		}
	}()
	for wi, w := range plan.Windows {
		if wi > 0 {
			raw, e = b.profileGuard(ctx, owned, "", "", "new-window", "-d", "-P", "-F", "#{session_id}|#{window_id}|#{pane_id}", "-t", owned.SessionID+":", "-n", w.Name, "-c", literalDir(w.Panes[0].Dir), "--", plan.Shell, "-i")
			if e != nil {
				return owned, e
			}
			next, err := receipt(raw)
			if err != nil || next.SessionID != owned.SessionID {
				if err == nil {
					err = core.Failure("protocol_error", "profile window created in another session")
				}
				return owned, err
			}
			owned.Windows = append(owned.Windows, next.Windows[0])
		}
		for pi, p := range w.Panes {
			current := &owned.Windows[wi]
			if pi > 0 {
				raw, e = b.profileGuard(ctx, owned, current.ID, current.Panes[pi-1], "split-window", "-d", "-"+p.Split, "-P", "-F", "#{session_id}|#{window_id}|#{pane_id}", "-t", current.Panes[pi-1], "-c", literalDir(p.Dir), "--", plan.Shell, "-i")
				if e != nil {
					return owned, e
				}
				next, err := receipt(raw)
				if err != nil || next.SessionID != owned.SessionID || next.Windows[0].ID != current.ID {
					if err == nil {
						err = core.Failure("protocol_error", "profile pane created in another window")
					}
					return owned, err
				}
				current.Panes = append(current.Panes, next.Windows[0].Panes[0])
			}
			if _, e = b.profileGuard(ctx, owned, current.ID, current.Panes[pi], "set-option", "-p", "-t", current.Panes[pi], "synchronize-panes", "off"); e != nil {
				return owned, e
			}
		}
		if w.Layout != "" {
			if _, e = b.profileGuard(ctx, owned, owned.Windows[wi].ID, "", "select-layout", "-t", owned.Windows[wi].ID, w.Layout); e != nil {
				return owned, e
			}
		}
	}
	// No profile input is sent before the whole topology has been built.
	for wi, w := range plan.Windows {
		for pi, p := range w.Panes {
			if p.Command == "" {
				continue
			}
			window := owned.Windows[wi]
			pane := window.Panes[pi]
			data := []byte(p.Command)
			const digits = "0123456789abcdef"
			for len(data) > 0 {
				chunk := data[:min(4096, len(data))]
				args := []string{"send-keys", "-H", "-t", pane, "--"}
				for _, c := range chunk {
					args = append(args, string([]byte{digits[c>>4], digits[c&15]}))
				}
				if _, e = b.profileGuard(ctx, owned, window.ID, pane, args...); e != nil {
					return owned, e
				}
				data = data[len(chunk):]
			}
			if _, e = b.profileGuard(ctx, owned, window.ID, pane, "send-keys", "-t", pane, "Enter"); e != nil {
				return owned, e
			}
		}
	}
	return owned, nil
}
func (b *TmuxBackend) attachProfile(ctx context.Context, owned profile.Applied, readOnly bool) error {
	if _, e := b.profileGuard(ctx, owned, "", "", "display-message", "-p", "PROFILE_OWNED"); e != nil {
		return core.Failure("conflict", "profile ownership changed before attachment")
	}
	args := []string{"attach-session", "-t", owned.SessionID}
	if readOnly {
		args = append(args, "-r")
	}
	// Keep the ownership check and attach in one server queue too.
	condition := "#{==:#{TXM_PROFILE_OWNER}," + owned.Owner + "}"
	bound := b.external
	bound.ctx = ctx
	return bound.interactive("if-shell", "-F", "-t", owned.SessionID+":", condition, tmuxCommand(args...), "show-options -t "+owned.SessionID+" __txm_profile_owner_mismatch")
}
