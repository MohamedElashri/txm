//go:build (!linux && !darwin) || !cgo

package backend

func nativeServerSupported() bool { return false }
