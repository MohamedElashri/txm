//go:build !linux && !darwin

package extension

import "github.com/MohamedElashri/txm/pkg/core"

type Identity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func Identify(string) (Identity, error) {
	return Identity{}, core.Failure("unsupported", "extension identity requires Unix")
}
func busy(error) bool { return false }
