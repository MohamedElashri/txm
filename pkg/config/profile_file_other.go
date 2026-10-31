//go:build !linux && !darwin

package config

import (
	"fmt"
	"os"
)

func openProfile(path string) (*os.File, error) {
	st, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("profile must be a regular file")
	}
	return os.Open(path)
}
