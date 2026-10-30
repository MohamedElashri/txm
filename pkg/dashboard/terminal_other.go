//go:build !linux && !darwin

package dashboard

// Runtime entry is explicitly unsupported outside tested Unix code paths.
func terminalGuard() func() { return func() {} }
