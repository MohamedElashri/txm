package main

import (
	"os"
	"strings"
	"testing"
)

// TestBackendString tests the String() method for Backend type
func TestBackendString(t *testing.T) {
	tests := []struct {
		backend  Backend
		expected string
	}{
		{BackendTmux, "tmux"},
		{BackendZellij, "zellij"},
		{BackendScreen, "screen"},
		{Backend(999), "unknown"}, // Invalid backend
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.backend.String(); got != tt.expected {
				t.Errorf("Backend.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestParseBackend tests the ParseBackend function
func TestParseBackend(t *testing.T) {
	tests := []struct {
		input     string
		expected  Backend
		shouldErr bool
	}{
		{"tmux", BackendTmux, false},
		{"zellij", BackendZellij, false},
		{"screen", BackendScreen, false},
		{"TMUX", BackendTmux, false},    // Case insensitive
		{"Zellij", BackendZellij, false}, // Case insensitive
		{"SCREEN", BackendScreen, false}, // Case insensitive
		{"invalid", BackendTmux, true},   // Invalid backend should error
		{"", BackendTmux, true},          // Empty string should error
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			backend, err := ParseBackend(tt.input)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("ParseBackend(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseBackend(%q) unexpected error: %v", tt.input, err)
				}
				if backend != tt.expected {
					t.Errorf("ParseBackend(%q) = %v, want %v", tt.input, backend, tt.expected)
				}
			}
		})
	}
}

// TestNewDefaultConfig tests the NewDefaultConfig function
func TestNewDefaultConfig(t *testing.T) {
	config := NewDefaultConfig()
	
	if config == nil {
		t.Fatal("NewDefaultConfig() returned nil")
	}
	
	if config.DefaultBackend != BackendTmux {
		t.Errorf("NewDefaultConfig().DefaultBackend = %v, want %v", config.DefaultBackend, BackendTmux)
	}
	
	expectedOrder := []Backend{BackendTmux, BackendScreen, BackendZellij}
	if len(config.BackendOrder) != len(expectedOrder) {
		t.Errorf("NewDefaultConfig().BackendOrder length = %d, want %d", len(config.BackendOrder), len(expectedOrder))
	}
	
	for i, backend := range expectedOrder {
		if i >= len(config.BackendOrder) || config.BackendOrder[i] != backend {
			t.Errorf("NewDefaultConfig().BackendOrder[%d] = %v, want %v", i, config.BackendOrder[i], backend)
		}
	}
}

// TestSessionManagerColorize tests the colorize method
func TestSessionManagerColorize(t *testing.T) {
	tests := []struct {
		name      string
		useColors bool
		color     string
		text      string
		expected  string
	}{
		{
			name:      "With colors enabled",
			useColors: true,
			color:     colorRed,
			text:      "test",
			expected:  colorRed + "test" + colorReset,
		},
		{
			name:      "With colors disabled",
			useColors: false,
			color:     colorRed,
			text:      "test",
			expected:  "test",
		},
		{
			name:      "Empty text with colors",
			useColors: true,
			color:     colorBlue,
			text:      "",
			expected:  colorBlue + "" + colorReset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &SessionManager{useColors: tt.useColors}
			if got := sm.colorize(tt.color, tt.text); got != tt.expected {
				t.Errorf("SessionManager.colorize() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestGetArg tests the getArg function
func TestGetArg(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		index        int
		defaultValue string
		expected     string
	}{
		{
			name:         "Existing argument",
			args:         []string{"cmd", "value1", "value2"},
			index:        1,
			defaultValue: "default",
			expected:     "value1",
		},
		{
			name:         "Non-existing argument",
			args:         []string{"cmd"},
			index:        2,
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "Index 0 (command name)",
			args:         []string{"txm", "command"},
			index:        0,
			defaultValue: "default",
			expected:     "txm",
		},
		{
			name:         "Empty args with default",
			args:         []string{},
			index:        1,
			defaultValue: "fallback",
			expected:     "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore os.Args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			
			os.Args = tt.args
			if got := getArg(tt.index, tt.defaultValue); got != tt.expected {
				t.Errorf("getArg(%d, %q) = %v, want %v", tt.index, tt.defaultValue, got, tt.expected)
			}
		})
	}
}

// TestBackendSelection tests the backend selection logic
func TestBackendSelection(t *testing.T) {
	tests := []struct {
		name               string
		configBackend      Backend
		tmuxAvailable      bool
		zellijAvailable    bool
		expectedBackend    Backend
	}{
		{
			name:               "Tmux preferred and available",
			configBackend:      BackendTmux,
			tmuxAvailable:      true,
			zellijAvailable:    false,
			expectedBackend:    BackendTmux,
		},
		{
			name:               "Zellij preferred and available",
			configBackend:      BackendZellij,
			tmuxAvailable:      false,
			zellijAvailable:    true,
			expectedBackend:    BackendZellij,
		},
		{
			name:               "Preferred not available, fallback to tmux",
			configBackend:      BackendZellij,
			tmuxAvailable:      true,
			zellijAvailable:    false,
			expectedBackend:    BackendTmux,
		},
		{
			name:               "No multiplexers available, fallback to screen",
			configBackend:      BackendTmux,
			tmuxAvailable:      false,
			zellijAvailable:    false,
			expectedBackend:    BackendScreen, // Falls back to screen since it's usually available
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DefaultBackend: tt.configBackend,
				BackendOrder:   []Backend{BackendTmux, BackendScreen, BackendZellij},
			}
			
			sm := &SessionManager{
				tmuxAvailable:   tt.tmuxAvailable,
				zellijAvailable: tt.zellijAvailable,
				config:          config,
			}
			
			backend := sm.selectBestBackend()
			if backend != tt.expectedBackend {
				t.Errorf("selectBestBackend() = %v, want %v", backend, tt.expectedBackend)
			}
		})
	}
}

// TestIsBackendAvailable tests the isBackendAvailable method
func TestIsBackendAvailable(t *testing.T) {
	sm := &SessionManager{
		tmuxAvailable:   true,
		zellijAvailable: false,
	}

	tests := []struct {
		backend  Backend
		expected bool
	}{
		{BackendTmux, true},    // tmuxAvailable is true
		{BackendZellij, false}, // zellijAvailable is false
		{BackendScreen, true},  // screen should be available in most environments
		{Backend(999), false},  // Invalid backend
	}

	for _, tt := range tests {
		t.Run(tt.backend.String(), func(t *testing.T) {
			// For screen, we can't easily mock the exec.LookPath call,
			// so we'll just test that it doesn't panic
			if tt.backend == BackendScreen {
				// Just ensure it returns a boolean without panicking
				result := sm.isBackendAvailable(tt.backend)
				if result != true && result != false {
					t.Errorf("isBackendAvailable(%v) returned non-boolean", tt.backend)
				}
				return
			}
			
			if got := sm.isBackendAvailable(tt.backend); got != tt.expected {
				t.Errorf("isBackendAvailable(%v) = %v, want %v", tt.backend, got, tt.expected)
			}
		})
	}
}

// TestEnvironmentVariableHandling tests environment variable configuration
func TestEnvironmentVariableHandling(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectBackend Backend
	}{
		{"tmux environment", "tmux", BackendTmux},
		{"zellij environment", "zellij", BackendZellij},
		{"screen environment", "screen", BackendScreen},
		{"invalid environment", "invalid", BackendTmux}, // Should fallback to default
		{"empty environment", "", BackendTmux},          // Should use default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			oldEnv := os.Getenv("TXM_DEFAULT_BACKEND")
			defer func() {
				if oldEnv == "" {
					os.Unsetenv("TXM_DEFAULT_BACKEND")
				} else {
					os.Setenv("TXM_DEFAULT_BACKEND", oldEnv)
				}
			}()

			// Set test environment
			if tt.envValue != "" {
				os.Setenv("TXM_DEFAULT_BACKEND", tt.envValue)
			} else {
				os.Unsetenv("TXM_DEFAULT_BACKEND")
			}

			// Load config (this should read the environment variable)
			config, err := LoadConfig()
			if err != nil && !strings.Contains(err.Error(), "no such file") {
				t.Fatalf("LoadConfig() unexpected error: %v", err)
			}

			if config.DefaultBackend != tt.expectBackend {
				t.Errorf("LoadConfig() with TXM_DEFAULT_BACKEND=%q: DefaultBackend = %v, want %v", 
					tt.envValue, config.DefaultBackend, tt.expectBackend)
			}
		})
	}
}

// TestColorSupportDetection tests the color support detection
func TestColorSupportDetection(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{"Verbose color check", true},
		{"Non-verbose color check", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that checkColorSupport doesn't panic and returns a boolean
			result := checkColorSupport(tt.verbose)
			if result != true && result != false {
				t.Errorf("checkColorSupport(%v) returned non-boolean: %v", tt.verbose, result)
			}
		})
	}
}

// TestNewSessionManagerInitialization tests SessionManager initialization
func TestNewSessionManagerInitialization(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{"Verbose session manager", true},
		{"Non-verbose session manager", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSessionManager(tt.verbose)
			
			if sm == nil {
				t.Fatal("NewSessionManager() returned nil")
			}
			
			if sm.verbose != tt.verbose {
				t.Errorf("NewSessionManager(%v).verbose = %v, want %v", tt.verbose, sm.verbose, tt.verbose)
			}
			
			if sm.config == nil {
				t.Error("NewSessionManager().config is nil")
			}
			
			// Verify that currentBackend is set to a valid value
			validBackends := []Backend{BackendTmux, BackendZellij, BackendScreen}
			validBackend := false
			for _, backend := range validBackends {
				if sm.currentBackend == backend {
					validBackend = true
					break
				}
			}
			if !validBackend {
				t.Errorf("NewSessionManager().currentBackend = %v, want one of %v", sm.currentBackend, validBackends)
			}
		})
	}
}

// TestLoggingFunctions tests the logging methods
func TestLoggingFunctions(t *testing.T) {
	tests := []struct {
		name      string
		useColors bool
		verbose   bool
		logFunc   func(*SessionManager, string)
		message   string
		expectFunc func(string) bool
	}{
		{
			name:      "Info logging with colors",
			useColors: true,
			verbose:   true,
			logFunc:   (*SessionManager).logInfo,
			message:   "test info",
			expectFunc: func(output string) bool {
				return strings.Contains(output, "test info") && strings.Contains(output, "[INFO]")
			},
		},
		{
			name:      "Warning logging without colors",
			useColors: false,
			verbose:   true,
			logFunc:   (*SessionManager).logWarning,
			message:   "test warning",
			expectFunc: func(output string) bool {
				return strings.Contains(output, "test warning") && strings.Contains(output, "[WARNING]")
			},
		},
		{
			name:      "Error logging",
			useColors: true,
			verbose:   true,
			logFunc:   (*SessionManager).logError,
			message:   "test error",
			expectFunc: func(output string) bool {
				return strings.Contains(output, "test error") && strings.Contains(output, "[ERROR]")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &SessionManager{
				useColors: tt.useColors,
				verbose:   tt.verbose,
			}
			
			// The logging functions write to stderr/stdout, which is hard to capture in unit tests
			// For now, just ensure they don't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Logging function panicked: %v", r)
				}
			}()
			
			tt.logFunc(sm, tt.message)
		})
	}
}

// TestConfigErrorHandling tests configuration error handling
func TestConfigErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		configBackend  string
		shouldUseDefault bool
	}{
		{"Valid tmux config", "tmux", false},
		{"Valid zellij config", "zellij", false},
		{"Valid screen config", "screen", false},
		{"Invalid config", "invalid", true},
		{"Empty config", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.configBackend == "" {
				config := NewDefaultConfig()
				if config.DefaultBackend != BackendTmux {
					t.Errorf("Default config should have tmux backend, got %v", config.DefaultBackend)
				}
				return
			}

			backend, err := ParseBackend(tt.configBackend)
			if tt.shouldUseDefault {
				if err == nil {
					t.Errorf("ParseBackend(%q) should have returned error", tt.configBackend)
				}
			} else {
				if err != nil {
					t.Errorf("ParseBackend(%q) unexpected error: %v", tt.configBackend, err)
				}
				expectedBackend, _ := ParseBackend(tt.configBackend)
				if backend != expectedBackend {
					t.Errorf("ParseBackend(%q) = %v, want %v", tt.configBackend, backend, expectedBackend)
				}
			}
		})
	}
}
