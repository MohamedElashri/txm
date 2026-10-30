//go:build (!linux && !darwin) || !cgo

package supervisor

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
)

func Supported() bool { return false }
func Run(context.Context, string) error {
	return core.Failure("unsupported", "managed execution requires Unix and CGO")
}
