package cmd

import (
	"testing"
)

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid Alphanumeric", "mySession1", true},
		{"Valid with Dashes", "my-session-1", true},
		{"Valid with Underscores", "my_session_1", true},
		{"Invalid spaces", "my session", false},
		{"Invalid special char !", "my!session", false},
		{"Invalid special char @", "my@session", false},
		{"Invalid special char /", "my/session", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidName(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}
