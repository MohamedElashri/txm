//go:build !linux && !darwin

package runtimepath

import "fmt"

func OpenState() (*Root, error)             { return nil, fmt.Errorf("private state unsupported") }
func (r *Root) Child(string) (*Root, error) { return nil, fmt.Errorf("private state unsupported") }
func (r *Root) ReadPrivate(string, int) ([]byte, error) {
	return nil, fmt.Errorf("private state unsupported")
}
func (r *Root) RemovePrivate(string) error { return fmt.Errorf("private state unsupported") }
func (r *Root) WritePrivate(string, []byte, bool) error {
	return fmt.Errorf("private state unsupported")
}
