//go:build !linux && !darwin

package profile

func searchable(string) error { return nil }
