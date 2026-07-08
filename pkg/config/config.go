package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BackendType represents the type of terminal multiplexer backend
type BackendType string

const (
	BackendTmux   BackendType = "tmux"
	BackendZellij BackendType = "zellij"
	BackendScreen BackendType = "screen"
	BackendNative BackendType = "native"
)

// ParseBackend parses a string into a BackendType
func ParseBackend(s string) (BackendType, error) {
	switch strings.ToLower(s) {
	case "tmux":
		return BackendTmux, nil
	case "zellij":
		return BackendZellij, nil
	case "screen":
		return BackendScreen, nil
	case "native":
		return BackendNative, nil
	default:
		return BackendTmux, fmt.Errorf("invalid backend: %s", s)
	}
}

// Config represents the configuration for txm
type Config struct {
	DefaultBackend BackendType
	BackendOrder    []BackendType
	ScrollbackSize  int
	LogRotationSize int
}

// NewDefaultConfig creates a new default configuration
func NewDefaultConfig() *Config {
	return &Config{
		DefaultBackend: BackendTmux,
		BackendOrder:    []BackendType{BackendTmux, BackendScreen, BackendZellij, BackendNative},
		ScrollbackSize:  65536,
		LogRotationSize: 10485760, // 10MB default
	}
}

// LoadConfig loads configuration from environment variables and config file
func LoadConfig() (*Config, error) {
	config := NewDefaultConfig()

	// Load config file first
	configFile := getConfigFilePath()
	if configFile != "" {
		if err := loadConfigFile(config, configFile); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("error loading config file: %v", err)
			}
		}
	}

	// Check environment variable last (highest priority)
	if envBackend := os.Getenv("TXM_DEFAULT_BACKEND"); envBackend != "" {
		if backend, err := ParseBackend(envBackend); err == nil {
			config.DefaultBackend = backend
		}
	}

	return config, nil
}

// getConfigFilePath returns the path to the config file
func getConfigFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

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

		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch strings.ToLower(key) {
				case "default_backend", "backend":
					if backend, err := ParseBackend(value); err == nil {
						config.DefaultBackend = backend
					}
				case "scrollbacksize", "scrollback_size":
					if size, err := strconv.Atoi(value); err == nil {
						config.ScrollbackSize = size
					}
				case "logrotationsize", "log_rotation_size":
					if size, err := strconv.Atoi(value); err == nil {
						config.LogRotationSize = size
					}
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
	content := fmt.Sprintf("# txm configuration file\n# Set the default backend (tmux, zellij, screen)\ndefault_backend=%s\nscrollback_size=%d\nlog_rotation_size=%d\n", config.DefaultBackend, config.ScrollbackSize, config.LogRotationSize)

	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}
