//go:build !linux && !darwin

package runtimepath

import (
	"fmt"
	"net"
	"os"
)

func Supported() bool { return false }
func Open() (*Root, error) {
	return nil, fmt.Errorf("native persistence is unsupported on this platform")
}
func (r *Root) Lock(string) (*os.File, error) { return nil, fmt.Errorf("unsupported platform") }
func (r *Root) SocketInfo(string) (os.FileInfo, error) {
	return nil, fmt.Errorf("unsupported platform")
}
func (r *Root) RemoveSocket(string, os.FileInfo) error { return fmt.Errorf("unsupported platform") }
func VerifyPeer(net.Conn) error                        { return fmt.Errorf("unsupported platform") }
func OpenLog(string) (*os.File, error) {
	return nil, fmt.Errorf("native logging is unsupported on this platform")
}
