//go:build !linux && !darwin

package extension

import "github.com/MohamedElashri/txm/pkg/core"

func ExecutableHash(string) (string, error) {
	return "", core.Failure("unsupported", "reviewed executable identity requires Unix")
}
