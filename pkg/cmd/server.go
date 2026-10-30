package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/MohamedElashri/txm/pkg/nativeserver"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server [session_name] [command...]",
	Short: "Internal native session server (Linux/macOS)", Hidden: true,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (retErr error) {
		instance, err := cmd.Flags().GetString("instance")
		if err != nil {
			return err
		}
		fd, err := cmd.Flags().GetInt("ready-fd")
		if err != nil {
			return err
		}
		if fd != -1 && fd != 3 {
			return fmt.Errorf("invalid native readiness descriptor")
		}
		var ready *os.File
		if fd == 3 {
			ready = os.NewFile(uintptr(fd), "native-readiness")
			defer func() { _ = ready.Close() }()
		}
		sent := false
		notify := func(err error) error {
			if sent {
				return nil
			}
			sent = true
			if ready == nil {
				return nil
			}
			message := ""
			if err != nil {
				message = err.Error()
				if len(message) > 1024 {
					message = message[:1024]
				}
			}
			return json.NewEncoder(ready).Encode(struct {
				InstanceID string `json:"instance_id"`
				Error      string `json:"error"`
			}{instance, message})
		}
		defer func() {
			if !sent {
				_ = notify(retErr)
			}
		}()
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		var log *rotatingFileWriter
		if filename := os.Getenv("TXM_LOG_FILE"); filename != "" {
			log, err = newRotatingFileWriter(filename, cfg.LogRotationSize)
			if err != nil {
				return err
			}
			defer func() { _ = log.Close() }()
		}
		opts := nativeserver.Options{Name: args[0], InstanceID: instance, Command: args[1:], ScrollbackLines: cfg.ScrollbackSize, Ready: notify}
		if log != nil {
			opts.Log = log
		}
		return nativeserver.Serve(cmd.Context(), opts)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().SetInterspersed(false)
	serverCmd.Flags().String("instance", "", "Internal native instance ID")
	serverCmd.Flags().Int("ready-fd", -1, "Internal readiness pipe")
	_ = serverCmd.Flags().MarkHidden("instance")
	_ = serverCmd.Flags().MarkHidden("ready-fd")
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

	f, err := runtimepath.OpenLog(filename)
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
		if e := r.rotate(); err == nil {
			err = e
		}
	}
	return n, err
}

func (r *rotatingFileWriter) rotate() error {
	if err := r.file.Close(); err != nil {
		return err
	}
	if err := os.Rename(r.filename, r.filename+".1"); err != nil {
		return err
	}
	f, err := runtimepath.OpenLog(r.filename)
	if err != nil {
		return err
	}
	r.file = f
	r.size = 0
	return nil
}

func (r *rotatingFileWriter) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	return r.file.Close()
}
