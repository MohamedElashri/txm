package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [session_name]",
	Short: "Create a new session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := validateName(name); err != nil {
			return err
		}

		if err := manager.Backend.CreateSession(name); err != nil {
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
	Use:   "attach [session_name]",
	Short: "Attach to an existing session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := validateName(name); err != nil {
			return err
		}

		if !manager.Backend.SessionExists(name) {
			logInstance.Error(fmt.Sprintf("Session '%s' does not exist", name))
			return nil
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
	Use:   "delete [session_name]",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
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
	Use:   "rename-session [old_name] [new_name]",
	Short: "Rename an existing session",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]
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
