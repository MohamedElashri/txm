//go:build !linux && !darwin

package metrics

import "errors"

func errWouldBlock() error { return errors.New("unsupported metrics locks") }
