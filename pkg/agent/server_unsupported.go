//go:build !linux && !darwin

package agent

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
)

func Serve(context.Context, string, func(error)) error {
	return core.Failure("unsupported", "coordinator transport unsupported")
}
func startCoordinator(context.Context) error {
	return core.Failure("unsupported", "coordinator transport unsupported")
}
func lockBusy(error) bool         { return false }
func staleCoordinator(error) bool { return false }
