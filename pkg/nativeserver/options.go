// Package nativeserver owns a single native PTY session and its framed clients.
package nativeserver

import "io"

type Options struct {
	Name, InstanceID string
	Command          []string
	ScrollbackLines  int
	Log              io.Writer
	// Ready is called once, on successful startup or with an early startup error.
	Ready func(error) error
}
