package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Backend represents the type of terminal multiplexer backend
type Backend int

const (
	BackendTmux Backend = iota
	BackendZellij
	BackendScreen
)

// String returns the string representation of the backend
func (b Backend) String() string {
	switch b {
	case BackendTmux:
		return "tmux"
	case BackendZellij:
		return "zellij"
	case BackendScreen:
		return "screen"
	default:
		return "unknown"
	}
}

// ParseBackend parses a string into a Backend type
func ParseBackend(s string) Backend {
	switch strings.ToLower(s) {
	case "tmux":
		return BackendTmux
	case "zellij":
		return BackendZellij
	case "screen":
		return BackendScreen
	default:
		return BackendTmux // Default to tmux
	}
}

// Config represents the configuration for txm
type Config struct {
	DefaultBackend Backend `json:"default_backend"`
	BackendOrder   []Backend `json:"backend_order"`
}

// NewDefaultConfig creates a new default configuration
func NewDefaultConfig() *Config {
	return &Config{
		DefaultBackend: BackendTmux,
		BackendOrder:   []Backend{BackendTmux, BackendZellij, BackendScreen},
	}
}

// LoadConfig loads configuration from environment variables and config file
func LoadConfig() (*Config, error) {
	config := NewDefaultConfig()

	// Load config file first
	configFile := getConfigFilePath()
	if configFile != "" {
		if err := loadConfigFile(config, configFile); err != nil {
			// If config file exists but has errors, return the error
			// If config file doesn't exist, continue with defaults
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("error loading config file: %v", err)
			}
		}
	}

	// Check environment variable last (highest priority)
	if envBackend := os.Getenv("TXM_DEFAULT_BACKEND"); envBackend != "" {
		config.DefaultBackend = ParseBackend(envBackend)
	}

	return config, nil
}

// getConfigFilePath returns the path to the config file
func getConfigFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check for various config file formats
	configPaths := []string{
		filepath.Join(homeDir, ".txm", "config"),
		filepath.Join(homeDir, ".txm", "config.txt"),
		filepath.Join(homeDir, ".txmrc"),
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// loadConfigFile loads configuration from a simple text file
func loadConfigFile(config *Config, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse simple key=value format
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch strings.ToLower(key) {
				case "default_backend", "backend":
					config.DefaultBackend = ParseBackend(value)
				}
			}
		}
	}

	return nil
}

// SaveConfig saves the configuration to a config file
func SaveConfig(config *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".txm")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	configFile := filepath.Join(configDir, "config")
	content := fmt.Sprintf("# txm configuration file\n# Set the default backend (tmux, zellij, screen)\ndefault_backend=%s\n", config.DefaultBackend.String())

	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}