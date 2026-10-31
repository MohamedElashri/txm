package cmd

import (
	"fmt"
	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"os"

	"github.com/spf13/cobra"
)

var createLogFile string
var attachReadOnly bool

var sessionCmd = &cobra.Command{
	Use:     "session",
	Aliases: []string{"s"},
	Short:   "Manage sessions",
}

var sessionCreateCmd = &cobra.Command{
	Use:     "create [session_name] [command...]",
	Aliases: []string{"c", "new"},
	Short:   "Create a new session",
	Args:    cobra.MinimumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := parseSessionArgs(args)
		if err != nil {
			return err
		}

		if err := validateName(name); err != nil {
			return err
		}
		if createLogFile != "" {
			if manager.Backend.Name() != "native" {
				return core.Failure("unsupported", "output logging is only supported by native")
			}
			restore, err := scopedEnvironment("TXM_LOG_FILE", createLogFile)
			if err != nil {
				return err
			}
			defer restore()
		}

		if err := manager.Backend.CreateSession(name, args[1:]...); err != nil {
			return fmt.Errorf("failed to create %s session '%s': %w", manager.Backend.Name(), name, err)
		}
		logInstance.Info(fmt.Sprintf("Session '%s' created with %s", name, manager.Backend.Name()))
		return nil
	},
}

var sessionListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all active sessions",
	Args:    cobra.NoArgs,
	RunE:    listSessions,
}

var sessionAttachCmd = &cobra.Command{
	Use:               "attach [session_name] [command...]",
	Aliases:           []string{"a"},
	Short:             "Attach to an existing session",
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		readOnly := attachReadOnly || os.Getenv("TXM_READ_ONLY") == "1"
		var observer interface{ AttachReadOnly(string) error }
		if readOnly {
			var ok bool
			observer, ok = manager.Backend.(interface{ AttachReadOnly(string) error })
			if !ok {
				return core.Failure("unsupported", "read-only attach is unsupported by this backend")
			}
		}
		var name string

		if len(args) == 0 {
			sessions, err := manager.Backend.GetSessions()
			if err != nil {
				return fmt.Errorf("failed to get sessions: %w", err)
			}

			if len(sessions) == 1 {
				name = sessions[0]
			} else if len(sessions) == 0 {
				if readOnly {
					return core.Failure("not_found", "no sessions to observe")
				}
				name = getSessionName("default")
				if err := validateName(name); err != nil {
					return err
				}
				logInstance.Info(fmt.Sprintf("No sessions found. Creating default session '%s'...", name))
				if err := manager.Backend.CreateSession(name); err != nil {
					return fmt.Errorf("failed to create default session: %w", err)
				}
			} else {
				return core.Failure("ambiguous_target", "multiple sessions exist; specify a target")
			}
		} else {
			var err error
			name, err = parseSessionArgs(args)
			if err != nil {
				return err
			}

			exists, err := backend.FindSession(manager.Backend, name, cmd.Context())
			if err != nil {
				return err
			}
			if !exists {
				if readOnly {
					return core.Failure("not_found", "session does not exist")
				}
				logInstance.Info(fmt.Sprintf("Session '%s' does not exist. Creating it...", name))
				if err := manager.Backend.CreateSession(name, args[1:]...); err != nil {
					return fmt.Errorf("failed to create session: %w", err)
				}
			} else if len(args) > 1 {
				return core.Failure("conflict", "session already exists; cannot run a new command")
			}
		}

		if readOnly {
			return observer.AttachReadOnly(name)
		}

		if err := manager.Backend.AttachSession(name); err != nil {
			return fmt.Errorf("failed to attach to %s session '%s': %w", manager.Backend.Name(), name, err)
		}
		return nil
	},
}

var sessionDetachCmd = &cobra.Command{
	Use:     "detach",
	Aliases: []string{"d"},
	Short:   "Detach from current session",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := manager.Backend.DetachSession(); err != nil {
			return fmt.Errorf("failed to detach from %s session: %w", manager.Backend.Name(), err)
		}
		logInstance.Info(fmt.Sprintf("Detached from %s session", manager.Backend.Name()))
		return nil
	},
}

var sessionDeleteCmd = &cobra.Command{
	Use:               "delete [session_name]",
	Aliases:           []string{"kill", "rm"},
	Short:             "Delete a session",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := parseSessionArgs(args)
		if err != nil {
			return err
		}

		if err := manager.Backend.KillSession(name); err != nil {
			return fmt.Errorf("failed to kill %s session '%s': %w", manager.Backend.Name(), name, err)
		}
		logInstance.Info(fmt.Sprintf("Killed %s session '%s'", manager.Backend.Name(), name))
		return nil
	},
}

var sessionRenameCmd = &cobra.Command{
	Use:               "rename [old_name] [new_name]",
	Aliases:           []string{"mv"},
	Short:             "Rename an existing session",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName, err := parseSessionArgs(args)
		if err != nil {
			return err
		}
		newName, err := parseSessionArgs(args[1:])
		if err != nil {
			return err
		}

		exists, err := backend.FindSession(manager.Backend, oldName, cmd.Context())
		if err != nil {
			return err
		}
		if !exists {
			return core.Failure("not_found", "session does not exist")
		}

		if err := validateName(newName); err != nil {
			return err
		}
		if err := manager.Backend.RenameSession(oldName, newName); err != nil {
			return fmt.Errorf("failed to rename %s session from '%s' to '%s': %w", manager.Backend.Name(), oldName, newName, err)
		}
		logInstance.Info(fmt.Sprintf("Renamed %s session from '%s' to '%s'", manager.Backend.Name(), oldName, newName))
		return nil
	},
}

var sessionNukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Remove all sessions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := manager.Backend.NukeAllSessions(); err != nil {
			return fmt.Errorf("failed to nuke all %s sessions: %w", manager.Backend.Name(), err)
		}
		logInstance.Info(fmt.Sprintf("Nuked all %s sessions", manager.Backend.Name()))
		return nil
	},
}

var sessionDumpCmd = &cobra.Command{
	Use:               "dump [session_name]",
	Hidden:            true,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE:              captureOutput,
}

func init() {
	sessionCmd.AddCommand(sessionCreateCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionAttachCmd)
	sessionCmd.AddCommand(sessionDetachCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionRenameCmd)
	sessionCmd.AddCommand(sessionNukeCmd)
	sessionCmd.AddCommand(sessionDumpCmd)
}
