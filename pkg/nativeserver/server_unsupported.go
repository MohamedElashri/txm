//go:build (!linux && !darwin) || !cgo

package nativeserver

import (
	"context"
	"fmt"
)

func Serve(_ context.Context, options Options) error {
	err := fmt.Errorf("native persistence requires Linux/macOS and CGO")
	if options.Ready != nil {
		_ = options.Ready(err)
	}
	return err
}
