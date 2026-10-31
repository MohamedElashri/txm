//go:build !linux && !darwin

package runtimepath

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
)

func HostID(context.Context) (string, error) {
	return "", core.Failure("unsupported", "private structured state is not verified on this platform")
}
