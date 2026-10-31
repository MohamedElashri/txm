//go:build (!linux && !darwin) || !cgo

package nativeprocess

import "fmt"

func WaitExited(int) error { return fmt.Errorf("native child observation requires Unix and CGO") }

func Exited(int) (bool, error) {
	return false, fmt.Errorf("native child observation requires Unix and CGO")
}
