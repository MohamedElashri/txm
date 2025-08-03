package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestSessionManager tests the SessionManager functionality
type testCase struct {
	name     string
	setup    func(t *testing.T) *SessionManager
	test     func(t *testing.T, sm *SessionManager)
	teardown func(t *testing.T, sm *SessionManager)
}

func TestSessionManager(t *testing.T) {
	tests := []testCase{
		{
			name: "New Session Manager Initialization",
			setup: func(t *testing.T) *SessionManager {
				return NewSessionManager(true)
			},
			test: func(t *testing.T, sm *SessionManager) {
				if sm == nil {
					t.Fatal("SessionManager should not be nil")
				}
				if !sm.verbose {
					t.Error("SessionManager should be verbose")
				}
			},
			teardown: func(t *testing.T, sm *SessionManager) {},
		},
		{
			name: "Session Creation and Deletion",
			setup: func(t *testing.T) *SessionManager {
				return NewSessionManager(true)
			},
			test: func(t *testing.T, sm *SessionManager) {
				testSession := "test-session"
				
				// Create session
				sm.createSession(testSession)
				
				// Verify session exists
				if !sm.sessionExists(testSession) {
					t.Errorf("Session %s should exist after creation", testSession)
				}
				
				// Kill session
				sm.killSession(testSession)
				
				// Verify session no longer exists
				if sm.sessionExists(testSession) {
					t.Errorf("Session %s should not exist after deletion", testSession)
				}
			},
			teardown: func(t *testing.T, sm *SessionManager) {
				// Clean up any remaining test sessions
				if sm.sessionExists("test-session") {
					sm.killSession("test-session")
				}
			},
		},
		{
			name: "Window Management",
			setup: func(t *testing.T) *SessionManager {
				sm := NewSessionManager(true)
				sm.createSession("test-session")
				return sm
			},
			test: func(t *testing.T, sm *SessionManager) {
				session := "test-session"
				window := "test-window"
				
				// Create new window
				sm.newWindow(session, window)
				
				// Test window navigation
				sm.nextWindow(session)
				sm.previousWindow(session)
				
				// Kill window
				sm.killWindow(session, window)
			},
			teardown: func(t *testing.T, sm *SessionManager) {
				sm.killSession("test-session")
			},
		},
		{
			name: "Session Attach/Detach",
			setup: func(t *testing.T) *SessionManager {
				sm := NewSessionManager(true)
				sm.createSession("test-session")
				return sm
			},
			test: func(t *testing.T, sm *SessionManager) {
				session := "test-session"
				
				// Test attach
				sm.attachSession(session)
				
				// Test detach
				sm.detachSession()
			},
			teardown: func(t *testing.T, sm *SessionManager) {
				sm.killSession("test-session")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sm := tc.setup(t)
			defer tc.teardown(t, sm)
			tc.test(t, sm)
		})
	}
}

// TestCommandExecution tests the command execution functionality
func TestCommandExecution(t *testing.T) {
	sm := NewSessionManager(true)

	// Set backend to tmux for this test
	sm.currentBackend = BackendTmux

	// Create a test session first to ensure tmux server is running
	sm.createSession("test-session")
	defer sm.killSession("test-session")

	tests := []struct {
		name    string
		cmd     []string
		wantErr bool
	}{
		{
			name:    "Valid tmux command",
			cmd:     []string{"has-session", "-t", "test-session"},
			wantErr: false,
		},
		{
			name:    "Invalid tmux command",
			cmd:     []string{"invalid-command"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.runTmuxCommand(tt.cmd...)
			if (err != nil) != tt.wantErr {
				t.Errorf("runTmuxCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestColorSupport tests the color support functionality
func TestColorSupport(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{
			name:    "Verbose color support check",
			verbose: true,
		},
		{
			name:    "Non-verbose color support check",
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just test that the function runs without error
			// Color support detection depends on environment
			result := checkColorSupport(tt.verbose)
			t.Logf("Color support detected: %v", result)
		})
	}
}

// TestLogging tests the logging functionality
func TestLogging(t *testing.T) {
	sm := NewSessionManager(true)
	
	// Capture stdout to verify log messages
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test info logging
	sm.logInfo("Test info message")
	
	// Test warning logging
	sm.logWarning("Test warning message")
	
	// Test error logging
	sm.logError("Test error message")

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Verify log messages
	if !strings.Contains(output, "Test info message") {
		t.Error("Info message not logged correctly")
	}
	if !strings.Contains(output, "Test warning message") {
		t.Error("Warning message not logged correctly")
	}
	if !strings.Contains(output, "Test error message") {
		t.Error("Error message not logged correctly")
	}
}

// TestEnvironmentPreservation tests the environment preservation functionality
func TestEnvironmentPreservation(t *testing.T) {
	// Set test environment variables
	testEnv := []string{
		"TERM=xterm-256color",
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}

	cmd := exec.Command("echo", "test")
	cmd.Env = testEnv
	preserveEnvironment(cmd)

	// Verify that important environment variables are preserved
	foundTerm := false
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, "TERM=") {
			foundTerm = true
			break
		}
	}

	if !foundTerm {
		t.Error("TERM environment variable not preserved")
	}

	// Check if the environment has at least the minimum required variables
	if len(cmd.Env) < len(testEnv) {
		t.Error("Not all environment variables were preserved")
	}
}

// TestArgumentHandling tests the argument handling functionality
func TestArgumentHandling(t *testing.T) {
	tests := []struct {
		name         string
		index        int
		defaultValue string
		args         []string
		want         string
	}{
		{
			name:         "Get existing argument",
			index:        1,
			defaultValue: "default",
			args:         []string{"cmd", "value"},
			want:         "value",
		},
		{
			name:         "Get non-existing argument",
			index:        2,
			defaultValue: "default",
			args:         []string{"cmd"},
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore os.Args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			
			os.Args = tt.args
			if got := getArg(tt.index, tt.defaultValue); got != tt.want {
				t.Errorf("getArg() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConfigFunctionality tests the configuration functionality
func TestConfigFunctionality(t *testing.T) {
	// Test config loading and default values
	config := NewDefaultConfig()
	if config.DefaultBackend != BackendTmux {
		t.Error("Default backend should be tmux")
	}
	
	// Test backend parsing
	if backend, err := ParseBackend("tmux"); err != nil || backend != BackendTmux {
		t.Error("Failed to parse tmux backend")
	}
	if backend, err := ParseBackend("zellij"); err != nil || backend != BackendZellij {
		t.Error("Failed to parse zellij backend")
	}
	if backend, err := ParseBackend("screen"); err != nil || backend != BackendScreen {
		t.Error("Failed to parse screen backend")
	}
	if _, err := ParseBackend("invalid"); err == nil {
		t.Error("Invalid backend should return an error")
	}
	
	// Test backend string representation
	if BackendTmux.String() != "tmux" {
		t.Error("Backend tmux string representation incorrect")
	}
	if BackendZellij.String() != "zellij" {
		t.Error("Backend zellij string representation incorrect")
	}
	if BackendScreen.String() != "screen" {
		t.Error("Backend screen string representation incorrect")
	}
}

// TestBackendSelection tests the backend selection logic
func TestBackendSelection(t *testing.T) {
	sm := NewSessionManager(false)
	
	// Test that backend selection works
	if sm.currentBackend == BackendTmux && !sm.tmuxAvailable {
		t.Error("Should not select tmux when not available")
	}
	
	// Test backend availability checking
	tmuxAvail := sm.isBackendAvailable(BackendTmux)
	zellijAvail := sm.isBackendAvailable(BackendZellij)
	screenAvail := sm.isBackendAvailable(BackendScreen)
	
	if tmuxAvail != sm.tmuxAvailable {
		t.Error("Tmux availability check inconsistent")
	}
	if zellijAvail != sm.zellijAvailable {
		t.Error("Zellij availability check inconsistent")
	}
	if !screenAvail {
		// Screen should be available in most test environments
		t.Log("Screen not available in test environment")
	}
}
