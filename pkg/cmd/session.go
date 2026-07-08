package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var createLogFile string
var attachReadOnly bool

var createCmd = &cobra.Command{
	Use:   "create [session_name] [command...]",
	Short: "Create a new session",
	Args:  cobra.MinimumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		name := getSessionName(args[0])
		if err := validateName(name); err != nil {
			return err
		}

		if createLogFile != "" {
			_ = os.Setenv("TXM_LOG_FILE", createLogFile)
		}

		if err := manager.Backend.CreateSession(name, args[1:]...); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to create %s session '%s': %v", manager.Backend.Name(), name, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Session '%s' created with %s", name, manager.Backend.Name()))
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active sessions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := manager.Backend.ListSessions(); err != nil {
			logInstance.Warning(fmt.Sprintf("No %s sessions found", manager.Backend.Name()))
		}
		return nil
	},
}

var attachCmd = &cobra.Command{
	Use:               "attach [session_name] [command...]",
	Short:             "Attach to an existing session",
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string

		if len(args) == 0 {
			sessions, err := manager.Backend.GetSessions()
			if err != nil {
				return fmt.Errorf("failed to get sessions: %v", err)
			}

			if len(sessions) == 1 {
				name = sessions[0]
			} else if len(sessions) == 0 {
				name = "default"
				logInstance.Info(fmt.Sprintf("No sessions found. Creating default session '%s'...", name))
				if err := manager.Backend.CreateSession(name); err != nil {
					return fmt.Errorf("failed to create default session: %v", err)
				}
			} else {
				return fmt.Errorf("multiple sessions exist, please specify one to attach to")
			}
		} else {
			name = getSessionName(args[0])
			if err := validateName(name); err != nil {
				return err
			}

			if !manager.Backend.SessionExists(name) {
				logInstance.Info(fmt.Sprintf("Session '%s' does not exist. Creating it...", name))
				if err := manager.Backend.CreateSession(name, args[1:]...); err != nil {
					return fmt.Errorf("failed to create session: %v", err)
				}
			} else if len(args) > 1 {
				return fmt.Errorf("session '%s' already exists, cannot run a new command", name)
			}
		}

		if attachReadOnly {
			_ = os.Setenv("TXM_READ_ONLY", "1")
		}

		if err := manager.Backend.AttachSession(name); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to attach to %s session '%s': %v", manager.Backend.Name(), name, err))
		}
		return nil
	},
}

var detachCmd = &cobra.Command{
	Use:   "detach",
	Short: "Detach from current session",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := manager.Backend.DetachSession(); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to detach from %s session: %v", manager.Backend.Name(), err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Detached from %s session", manager.Backend.Name()))
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:               "delete [session_name]",
	Short:             "Delete a session",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := getSessionName(args[0])
		if err := validateName(name); err != nil {
			return err
		}

		if err := manager.Backend.KillSession(name); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to kill %s session '%s': %v", manager.Backend.Name(), name, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Killed %s session '%s'", manager.Backend.Name(), name))
		return nil
	},
}

var renameSessionCmd = &cobra.Command{
	Use:               "rename-session [old_name] [new_name]",
	Short:             "Rename an existing session",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := getSessionName(args[0])
		newName := getSessionName(args[1])
		if err := validateName(oldName); err != nil {
			return err
		}
		if err := validateName(newName); err != nil {
			return err
		}

		if !manager.Backend.SessionExists(oldName) {
			logInstance.Error(fmt.Sprintf("Session '%s' does not exist", oldName))
			return nil
		}

		if err := manager.Backend.RenameSession(oldName, newName); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to rename %s session from '%s' to '%s': %v", manager.Backend.Name(), oldName, newName, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Renamed %s session from '%s' to '%s'", manager.Backend.Name(), oldName, newName))
		return nil
	},
}

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Remove all sessions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := manager.Backend.NukeAllSessions(); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to nuke all %s sessions: %v", manager.Backend.Name(), err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Nuked all %s sessions", manager.Backend.Name()))
		return nil
	},
}

var dumpCmd = &cobra.Command{
	Use:               "dump [session_name]",
	Hidden:            true,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := getSessionName(args[0])
		if err := validateName(name); err != nil {
			return err
		}

		out, err := manager.Backend.DumpSession(name)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

var generateSshConfigCmd = &cobra.Command{
	Use:   "generate-ssh-config",
	Short: "Generate SSH config snippet for seamless remote workflows",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		config := `
# Add this to your ~/.ssh/config for seamless session persistence

Host d.*
    HostName 192.168.1.xxx

    # Automatically attach to a txm session on the remote server
    # named after the remote host we're connecting to (%h).
    RemoteCommand txm attach %h
    RequestTTY yes

    # Multiplex multiple PTY sessions to a single server over one connection
    ControlMaster auto
    ControlPath ~/.ssh/cm-%r@%h:%p
    ControlPersist 10m
`
		fmt.Println(strings.TrimSpace(config))
	},
}
