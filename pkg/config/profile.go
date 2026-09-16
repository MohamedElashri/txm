package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Profile represents a declarative session layout
type Profile struct {
	Name    string   `toml:"name"`
	Backend string   `toml:"backend,omitempty"`
	Windows []Window `toml:"window"`
}

type Window struct {
	Name   string `toml:"name"`
	Layout string `toml:"layout,omitempty"`
	Panes  []Pane `toml:"pane"`
}

type Pane struct {
	Command string `toml:"command,omitempty"`
	Dir     string `toml:"dir,omitempty"`
	Split   string `toml:"split,omitempty"` // "v" or "h"
}

// LoadProfile reads and parses a TOML profile
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var p Profile
	err = toml.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
