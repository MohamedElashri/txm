//go:build !linux

package dashboard

// Dashboard runtime is disabled until the platform's TTY guard is verified.
const UnverifiedNativeContext = "unverified"

func CurrentNativeInstance() string { return UnverifiedNativeContext }
