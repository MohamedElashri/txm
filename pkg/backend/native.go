package backend

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

type NativeBackend struct{}

func NewNativeBackend() *NativeBackend {
	return &NativeBackend{}
}

func (b *NativeBackend) Name() string {
	return "native"
}

func (b *NativeBackend) IsAvailable() bool {
	return true
}

func getSocketPath(name string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("txm-%s.sock", name))
}

func (b *NativeBackend) SessionExists(name string) bool {
	_, err := os.Stat(getSocketPath(name))
	return err == nil
}

func (b *NativeBackend) CreateSession(name string, command ...string) error {
	if b.SessionExists(name) {
		return fmt.Errorf("session %s already exists", name)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	args := []string{"server", name}
	if len(command) > 0 {
		args = append(args, command...)
	}

	cmd := exec.Command(exe, args...)
	setSysProcAttr(cmd)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start native server: %v", err)
	}

	// Wait for socket to appear
	for i := 0; i < 20; i++ {
		if b.SessionExists(name) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("server started but socket not created in time")
}

func (b *NativeBackend) ListSessions() error {
	sessions, err := b.GetSessions()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		conn, err := net.Dial("unix", getSocketPath(s))
		if err == nil {
			_, _ = conn.Write([]byte{0x04}) // Status query
			buf := make([]byte, 1)
			_, err = conn.Read(buf)
			_ = conn.Close()
			if err == nil {
				fmt.Printf("%s [Attached: %d]\n", s, buf[0])
				continue
			}
		}
		fmt.Println(s)
	}
	return nil
}

func (b *NativeBackend) DumpSession(name string) (string, error) {
	if !b.SessionExists(name) {
		return "", fmt.Errorf("session %s does not exist", name)
	}

	conn, err := net.Dial("unix", getSocketPath(name))
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	_, _ = io.WriteString(conn, "\x05")
	buf, _ := io.ReadAll(conn)
	return string(buf), nil
}

func (b *NativeBackend) GetSessions() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(os.TempDir(), "txm-*.sock"))
	if err != nil {
		return nil, err
	}

	var sessions []string
	for _, f := range files {
		base := filepath.Base(f)
		name := strings.TrimPrefix(base, "txm-")
		name = strings.TrimSuffix(name, ".sock")
		sessions = append(sessions, name)
	}
	return sessions, nil
}

func (b *NativeBackend) AttachSession(name string) error {
	if !b.SessionExists(name) {
		return fmt.Errorf("session %s does not exist", name)
	}

	conn, err := net.Dial("unix", getSocketPath(name))
	if err != nil {
		return fmt.Errorf("failed to connect to session: %v", err)
	}
	defer func() { _ = conn.Close() }()

	_, _ = conn.Write([]byte{0x00}) // Identify as attach

	// Put terminal in raw mode
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	// Handle window resize
	watchWindowSize(fd, conn)

	errChan := make(chan error, 1)

	// Copy from conn to stdout
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		errChan <- err
	}()

	readOnly := os.Getenv("TXM_READ_ONLY") == "1"

	// Copy from stdin to conn (wrapped in data packets)
	if !readOnly {
		go func() {
			buf := make([]byte, 1024)
			for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				errChan <- err
				return
			}

			// Scan for Ctrl+\ (0x1C)
			detachIdx := -1
			for i := 0; i < n; i++ {
				if buf[i] == 0x1C {
					detachIdx = i
					break
				}
			}

			if detachIdx != -1 {
				if detachIdx > 0 {
					payload := append([]byte{0x01}, buf[:detachIdx]...)
					if _, err := conn.Write(payload); err != nil {
						errChan <- err
						return
					}
				}
				errChan <- nil // Gracefully detach
				return
			}
			
			payload := append([]byte{0x01}, buf[:n]...)
			if _, err := conn.Write(payload); err != nil {
				errChan <- err
				return
			}
		}
	}()
	}

	<-errChan
	return nil
}

func (b *NativeBackend) DetachSession() error {
	return fmt.Errorf("to detach from a native session, close the terminal window or use a detach sequence (WIP)")
}

func (b *NativeBackend) KillSession(name string) error {
	if !b.SessionExists(name) {
		return fmt.Errorf("session %s does not exist", name)
	}
	
	conn, err := net.Dial("unix", getSocketPath(name))
	if err != nil {
		_ = os.Remove(getSocketPath(name))
		return nil
	}
	defer func() { _ = conn.Close() }()
	
	_, _ = conn.Write([]byte{0x03})
	return nil
}

func (b *NativeBackend) RenameSession(oldName, newName string) error {
	return fmt.Errorf("renaming sessions is not supported by the native backend")
}

func (b *NativeBackend) NewWindow(session, name string) error {
	return fmt.Errorf("window management is not supported by the native backend")
}

func (b *NativeBackend) ListWindows(session string) error {
	return fmt.Errorf("window management is not supported by the native backend")
}

func (b *NativeBackend) KillWindow(session, window string) error {
	return fmt.Errorf("window management is not supported by the native backend")
}

func (b *NativeBackend) NextWindow(session string) error {
	return fmt.Errorf("window management is not supported by the native backend")
}

func (b *NativeBackend) PreviousWindow(session string) error {
	return fmt.Errorf("window management is not supported by the native backend")
}

func (b *NativeBackend) RenameWindow(session, oldName, newName string) error {
	return fmt.Errorf("window management is not supported by the native backend")
}

func (b *NativeBackend) SplitWindow(session, window, direction string) error {
	return fmt.Errorf("window management is not supported by the native backend")
}

func (b *NativeBackend) ListPanes(session, window string) error {
	return fmt.Errorf("pane management is not supported by the native backend")
}

func (b *NativeBackend) KillPane(session, window, pane string) error {
	return fmt.Errorf("pane management is not supported by the native backend")
}

func (b *NativeBackend) Exec(session, window, pane, command string) error {
	if !b.SessionExists(session) {
		return fmt.Errorf("session %s does not exist", session)
	}

	conn, err := net.Dial("unix", getSocketPath(session))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	payload := append([]byte{0x01}, []byte(command+"\n")...)
	_, err = conn.Write(payload)
	return err
}

func (b *NativeBackend) NukeAllSessions() error {
	sessions, _ := b.GetSessions()
	for _, s := range sessions {
		_ = b.KillSession(s)
	}
	return nil
}
