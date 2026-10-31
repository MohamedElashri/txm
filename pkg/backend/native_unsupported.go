//go:build !linux && !darwin

package backend

import (
	"context"
	"fmt"
	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
)

func staleNativeSocket(error) bool  { return false }
func ownedLegacySocket(string) bool { return false }
func legacyNativeNames() []string   { return nil }
func startNative(context.Context, string, string, []string) error {
	return fmt.Errorf("native persistence is unsupported on this platform")
}
func attachNative(*p.Client, bool) error {
	return fmt.Errorf("native persistence is unsupported on this platform")
}

func startNativeInDir(ctx context.Context, name, id string, command []string, dir string) error {
	return startNative(ctx, name, id, command)
}
