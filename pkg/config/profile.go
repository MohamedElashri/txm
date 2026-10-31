package config

import (
	"bytes"
	"github.com/MohamedElashri/txm/pkg/core"
	"io"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Profile represents a declarative session layout
type Profile struct {
	Name      string   `toml:"name"`
	Backend   string   `toml:"backend,omitempty"`
	Windows   []Window `toml:"window"`
	SourceDir string   `toml:"-"`
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
	f, err := openProfile(path)
	if err != nil {
		return nil, core.Failure("invalid_profile", "could not open a regular profile file")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, core.Failure("invalid_profile", "profile exceeds 1 MiB or could not be read")
	}

	var p Profile
	err = toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&p)
	if err != nil {
		return nil, core.Failure("invalid_profile", "invalid profile TOML or unknown fields")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	p.SourceDir = filepath.Dir(abs)
	return &p, nil
}
