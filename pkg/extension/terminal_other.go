//go:build !linux && !darwin

package extension

func TerminalGuard() func() { return func() {} }
