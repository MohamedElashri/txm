//go:build !linux && !darwin

package runtimepath

import (
	"context"
	"fmt"
)

func OpenExistingState() (*Root, error) { return nil, fmt.Errorf("private state unsupported") }
func OpenExisting() (*Root, error)      { return nil, fmt.Errorf("private runtime unsupported") }
func ExistingHostID(context.Context) (string, error) {
	return "", fmt.Errorf("private identity unsupported")
}
func (r *Root) ChildExisting(string) (*Root, error) {
	return nil, fmt.Errorf("private state unsupported")
}
