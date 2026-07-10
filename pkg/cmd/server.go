package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/spf13/cobra"
	"go.mitchellh.com/libghostty"
)

var serverCmd = &cobra.Command{
	Use:    "server [session_name] [command...]",
	Short:  "Internal command to run the native session server",
	Hidden: true,
	Args:   cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, _ := getManager()
		scrollbackSize := 65536
		logRotationSize := 10485760
		if mgr != nil && mgr.Config != nil {
			if mgr.Config.ScrollbackSize > 0 {
				scrollbackSize = mgr.Config.ScrollbackSize
			}
			if mgr.Config.LogRotationSize > 0 {
				logRotationSize = mgr.Config.LogRotationSize
			}
		}

		term, err := libghostty.NewTerminal(
			libghostty.WithSize(80, 24),
			libghostty.WithMaxScrollback(uint(scrollbackSize)),
		)
		if err != nil {
			return fmt.Errorf("failed to create libghostty terminal: %v", err)
		}
		defer term.Close()

		logFile := os.Getenv("TXM_LOG_FILE")
		var logWriter *rotatingFileWriter
		if logFile != "" {
			var err error
			logWriter, err = newRotatingFileWriter(logFile, logRotationSize)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to open log file: %v\n", err)
			} else {
				defer func() { _ = logWriter.Close() }()
			}
		}

		session := args[0]
		socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("txm-%s.sock", session))

		_ = os.Remove(socketPath)

		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			return err
		}
		defer func() { _ = listener.Close() }()
		defer func() { _ = os.Remove(socketPath) }()

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "bash"
		}
		
		var shellCmd *exec.Cmd
		if len(args) > 1 {
			shellCmd = exec.Command(args[1], args[2:]...)
		} else {
			shellCmd = exec.Command(shell)
		}
		shellCmd.Env = os.Environ()

		ptmx, err := pty.Start(shellCmd)
		if err != nil {
			return err
		}
		defer func() { _ = ptmx.Close() }()

		var connsMutex sync.Mutex
		var conns []net.Conn
		var termMutex sync.Mutex

		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := ptmx.Read(buf)
				if err != nil {
					break
				}
				
				termMutex.Lock()
				_, _ = term.Write(buf[:n])
				termMutex.Unlock()
				
				if logWriter != nil {
					_, _ = logWriter.Write(buf[:n])
				}

				connsMutex.Lock()
				for _, c := range conns {
					_ = c.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
					if _, err := c.Write(buf[:n]); err != nil {
						_ = c.Close()
					}
				}
				connsMutex.Unlock()
			}
			_ = listener.Close()
		}()

		for {
			conn, err := listener.Accept()
			if err != nil {
				break
			}

			go func(c net.Conn) {
				buf := make([]byte, 4096)
				n, err := c.Read(buf)
				if err != nil || n == 0 {
					_ = c.Close()
					return
				}

				if buf[0] == 0x04 {
					connsMutex.Lock()
					count := len(conns)
					connsMutex.Unlock()
					_, _ = c.Write([]byte{byte(count)})
					_ = c.Close()
					return
				} else if buf[0] == 0x05 {
					termMutex.Lock()
					f, err := libghostty.NewFormatter(term, libghostty.WithFormatterFormat(libghostty.FormatterFormatVT))
					var output string
					if err == nil {
						output, _ = f.FormatString()
						f.Close()
					}
					termMutex.Unlock()
					if err == nil {
						_, _ = c.Write([]byte(output))
					}
					_ = c.Close()
					return
				} else if buf[0] == 0x00 {
					termMutex.Lock()
					f, err := libghostty.NewFormatter(term, libghostty.WithFormatterFormat(libghostty.FormatterFormatVT))
					var output string
					if err == nil {
						output, _ = f.FormatString()
						f.Close()
					}
					termMutex.Unlock()
					
					if err == nil {
						_, _ = c.Write([]byte(output))
					}
					
					connsMutex.Lock()
					conns = append(conns, c)
					connsMutex.Unlock()

					defer func() {
						_ = c.Close()
						connsMutex.Lock()
						for i, existing := range conns {
							if existing == c {
								conns = append(conns[:i], conns[i+1:]...)
								break
							}
						}
						connsMutex.Unlock()
					}()

					processPacket := func(data []byte) {
						if data[0] == 0x01 {
							_, _ = ptmx.Write(data[1:])
						} else if data[0] == 0x02 && len(data) >= 5 {
							w := (uint16(data[1]) << 8) | uint16(data[2])
							h := (uint16(data[3]) << 8) | uint16(data[4])
							_ = pty.Setsize(ptmx, &pty.Winsize{
								Rows: uint16(h),
								Cols: uint16(w),
							})
							
							termMutex.Lock()
							_ = term.Resize(w, h, 0, 0)
							termMutex.Unlock()
						} else if data[0] == 0x03 {
							_ = shellCmd.Process.Kill()
						}
					}

					if n > 1 {
						processPacket(buf[1:n])
					}

					for {
						n, err := c.Read(buf)
						if err != nil {
							break
						}
						if n > 0 {
							processPacket(buf[:n])
						}
					}
				} else {
					_ = c.Close()
				}
			}(conn)
		}

		_ = shellCmd.Wait()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().SetInterspersed(false)
}

type rotatingFileWriter struct {
	filename string
	maxSize  int
	file     *os.File
	size     int
	mu       sync.Mutex
}

func newRotatingFileWriter(filename string, maxSize int) (*rotatingFileWriter, error) {
	if filename == "" {
		return nil, nil
	}
	var size int
	info, err := os.Stat(filename)
	if err == nil {
		size = int(info.Size())
	}

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &rotatingFileWriter{
		filename: filename,
		maxSize:  maxSize,
		file:     f,
		size:     size,
	}, nil
}

func (r *rotatingFileWriter) Write(p []byte) (n int, err error) {
	if r == nil {
		return len(p), nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	n, err = r.file.Write(p)
	r.size += n

	if r.size >= r.maxSize {
		r.rotate()
	}
	return n, err
}

func (r *rotatingFileWriter) rotate() {
	_ = r.file.Close()
	_ = os.Rename(r.filename, r.filename+".1")
	f, err := os.OpenFile(r.filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		r.file = f
		r.size = 0
	}
}

func (r *rotatingFileWriter) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	return r.file.Close()
}
