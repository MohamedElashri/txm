//go:build (!linux && !darwin) || !cgo

package metrics

import "github.com/MohamedElashri/txm/pkg/core"

func Supported() bool { return false }
func probe(int) (counter, error) {
	return counter{}, core.Failure("unsupported", "metrics require Linux/macOS and CGO")
}
