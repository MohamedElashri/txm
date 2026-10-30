//go:build !linux && !darwin

package dashboard

func CurrentNativeInstance() string { return UnverifiedNativeContext }
