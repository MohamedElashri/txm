package backend

import (
	"context"
	"os"
	"os/exec"

	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/logger"
)

// TerminalMultiplexer defines the interface for all backend implementations
type TerminalMultiplexer interface {
	core.Reader
	IsAvailable() bool
	Name() string

	// Session Management
	SessionExists(name string) bool
	CreateSession(name string, command ...string) error
	ListSessions() error
	GetSessions() ([]string, error)
	DumpSession(name string) (string, error)
	AttachSession(name string) error
	DetachSession() error
	KillSession(name string) error
	RenameSession(oldName, newName string) error
	NukeAllSessions() error

	// Window Management
	NewWindow(session, name string) error
	ListWindows(session string) error
	KillWindow(session, window string) error
	NextWindow(session string) error
	PreviousWindow(session string) error
	RenameWindow(session, oldName, newName string) error
	SplitWindow(session, window, direction string) error

	// Pane Management
	ListPanes(session, window string) error
	KillPane(session, window, pane string) error
	Exec(session, window, pane, command string) error
}

// preserveEnvironment ensures proper environment variables are passed to subprocess
func preserveEnvironment(cmd *exec.Cmd) {
	env := os.Environ()

	preserveVars := []string{
		"PATH", "HOME", "USER", "SHELL", "TERM", "DISPLAY",
		"LANG", "LC_ALL", "XDG_RUNTIME_DIR", "TMPDIR",
		"SSH_AUTH_SOCK", "SSH_AGENT_PID",
	}

	for _, varName := range preserveVars {
		if val := os.Getenv(varName); val != "" {
			found := false
			for i, envVar := range env {
				if len(envVar) > len(varName)+1 && envVar[:len(varName)+1] == varName+"=" {
					env[i] = varName + "=" + val
					found = true
					break
				}
			}
			if !found {
				env = append(env, varName+"="+val)
			}
		}
	}

	cmd.Env = env
}

// Manager handles the currently selected backend
type Manager struct {
	Config  *config.Config
	Logger  *logger.Logger
	Backend TerminalMultiplexer
}

// NewManager creates a new backend manager
func NewManager(cfg *config.Config, log *logger.Logger, ctx ...context.Context) *Manager {
	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}
	backends := map[config.BackendType]TerminalMultiplexer{
		config.BackendTmux:   NewTmuxBackend(ctx...),
		config.BackendZellij: NewZellijBackend(ctx...),
		config.BackendScreen: NewScreenBackend(ctx...),
		config.BackendNative: NewNativeBackend(ctx...),
	}

	var selectedBackend TerminalMultiplexer

	// 1. Check default backend
	if b, ok := backends[cfg.DefaultBackend]; ok && b.IsAvailable() {
		selectedBackend = b
	} else {
		// 2. Check fallback order
		for _, bt := range cfg.BackendOrder {
			if b, ok := backends[bt]; ok && b.IsAvailable() {
				selectedBackend = b
				break
			}
		}
	}

	// 3. Ultimate fallback
	if selectedBackend == nil {
		if backends[config.BackendTmux].IsAvailable() {
			selectedBackend = backends[config.BackendTmux]
		} else if backends[config.BackendZellij].IsAvailable() {
			selectedBackend = backends[config.BackendZellij]
		} else if backends[config.BackendScreen].IsAvailable() {
			selectedBackend = backends[config.BackendScreen]
		} else {
			selectedBackend = backends[config.BackendNative]
		}
	}

	return &Manager{
		Config:  cfg,
		Logger:  log,
		Backend: selectedBackend,
	}
}

func (m *Manager) CheckAvailability() error {
	if m.Backend == nil || !m.Backend.IsAvailable() {
		return core.Failure("backend_unavailable", "no available backend (native, tmux, zellij, screen)")
	}
	return nil
}

// FindSession is a compatibility wrapper over typed, exact, error-aware lookup.
func FindSession(b TerminalMultiplexer, name string, contexts ...context.Context) (bool, error) {
	ctx := context.Background()
	if len(contexts) > 0 && contexts[0] != nil {
		ctx = contexts[0]
	}
	_, err := b.Lookup(ctx, name)
	if err != nil && core.Normalize(err).Code == "not_found" {
		return false, nil
	}
	return err == nil, err
}

// NewBackend is strict: it never selects a fallback adapter.
func NewBackend(name string, ctx context.Context) (TerminalMultiplexer, error) {
	switch name {
	case "native":
		return NewNativeBackend(ctx), nil
	case "tmux":
		return NewTmuxBackend(ctx), nil
	case "zellij":
		return NewZellijBackend(ctx), nil
	case "screen":
		return NewScreenBackend(ctx), nil
	}
	return nil, core.Failure("invalid_usage", "backend must be native, tmux, zellij or screen")
}
func BackendNames() []string { return []string{"native", "screen", "tmux", "zellij"} }
