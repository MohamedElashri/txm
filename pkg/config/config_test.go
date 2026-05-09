package config

import (
	"testing"
)

func TestParseBackend(t *testing.T) {
	tests := []struct {
		input       string
		expected    BackendType
		expectError bool
	}{
		{"tmux", BackendTmux, false},
		{"zellij", BackendZellij, false},
		{"screen", BackendScreen, false},
		{"TMUX", BackendTmux, false},
		{"invalid", BackendTmux, true},
		{"", BackendTmux, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseBackend(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for input %q, got nil", tt.input)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("ParseBackend(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
