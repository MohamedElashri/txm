// Package metrics samples only an explicitly opted-in, unreaped managed child.
package metrics

import (
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"time"
)

const MaxSamplers = 32
const MaxPerID = 1 << 20
const MaxTotal = 32 << 20
const sampleBudget = MaxTotal - (2 << 20) // index and atomic-publication headroom
const maxHistory = MaxPerID / 2           // old + replacement remain within the per-ID limit
const maxEntries = 1024
const maxSamples = 2048
const MaxAge = 24 * time.Hour

type Config struct {
	IntervalSeconds int `json:"interval_seconds"`
}

func (c Config) Validate() error {
	if c.IntervalSeconds < 2 || c.IntervalSeconds > 60 {
		return core.Failure("invalid_usage", "metrics interval must be 2..60 seconds")
	}
	return nil
}

type Owner struct {
	ManagedID  string `json:"managed_id"`
	InstanceID string `json:"instance_id"`
	Backend    string `json:"backend"`
}

func (o Owner) valid() bool {
	return runtimepath.ValidID(o.ManagedID) && runtimepath.ValidID(o.InstanceID) && o.ManagedID != o.InstanceID && (o.Backend == "native" || o.Backend == "tmux")
}

type Sample struct {
	At            time.Time `json:"at"`
	Scope         string    `json:"scope"`
	Source        string    `json:"source"`
	State         string    `json:"state"`
	StartIdentity string    `json:"start_identity"`
	CPUPercent    *float64  `json:"cpu_percent"`
	RSSBytes      *uint64   `json:"rss_bytes"`
}
type View struct {
	Owner     Owner   `json:"owner"`
	Latest    *Sample `json:"latest"`
	Samples   int     `json:"samples"`
	Freshness string  `json:"freshness"`
}
type counter struct {
	Parent   int
	Identity string
	CPU      time.Duration
	RSS      uint64
	At       time.Time
}

func project(previous, current counter) Sample {
	s := Sample{At: current.At.UTC(), Scope: "child_pid", Source: "kernel child process counters", State: "available", StartIdentity: current.Identity, RSSBytes: &current.RSS}
	if previous.Identity == current.Identity && current.CPU >= previous.CPU && current.At.After(previous.At) {
		v := 100 * float64(current.CPU-previous.CPU) / float64(current.At.Sub(previous.At))
		s.CPUPercent = &v
	}
	return s
}
