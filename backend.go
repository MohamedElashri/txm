package main

import (
	"os"
	"os/exec"
)

// Backend management functions

// selectBestBackend determines the best available backend based on config and availability
func (sm *SessionManager) selectBestBackend() Backend {
	// First try the configured default backend if it's available
	if sm.isBackendAvailable(sm.config.DefaultBackend) {
		return sm.config.DefaultBackend
	}

	// Fall back to the order specified in config
	for _, backend := range sm.config.BackendOrder {
		if sm.isBackendAvailable(backend) {
			return backend
		}
	}

	// Ultimate fallback to tmux
	return BackendTmux
}

// isBackendAvailable checks if a specific backend is available on the system
func (sm *SessionManager) isBackendAvailable(backend Backend) bool {
	switch backend {
	case BackendTmux:
		return sm.tmuxAvailable
	case BackendZellij:
		return sm.zellijAvailable
	case BackendScreen:
		if _, err := exec.LookPath("screen"); err == nil {
			return true
		}
		return false
	default:
		return false
	}
}

// preserveEnvironment ensures proper environment variables are passed to subprocess
func preserveEnvironment(cmd *exec.Cmd) {
	env := os.Environ()
	
	// Preserve key environment variables
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