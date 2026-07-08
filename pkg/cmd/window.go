package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var newWindowCmd = &cobra.Command{
	Use:   "new [session_name] [window_name]",
	Short: "Create a new window/tab",
	Args:  cobra.RangeArgs(1, 2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := getSessionName(args[0])
		if err := validateName(session); err != nil {
			return err
		}

		windowName := ""
		if len(args) > 1 {
			windowName = args[1]
			if err := validateName(windowName); err != nil {
				return err
			}
		}

		if err := manager.Backend.NewWindow(session, windowName); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to create window in session '%s': %v", session, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Created window in session '%s'", session))
		return nil
	},
}

var listWindowsCmd = &cobra.Command{
	Use:   "list [session_name]",
	Short: "List windows in a session",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := getSessionName(args[0])
		if err := validateName(session); err != nil {
			return err
		}

		if err := manager.Backend.ListWindows(session); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to list windows in session '%s': %v", session, err))
		}
		return nil
	},
}

var killWindowCmd = &cobra.Command{
	Use:   "kill [session_name] [window_name]",
	Short: "Remove a window",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := getSessionName(args[0])
		window := args[1]
		if err := validateName(session); err != nil {
			return err
		}
		if err := validateName(window); err != nil {
			return err
		}

		if err := manager.Backend.KillWindow(session, window); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to kill window '%s' in session '%s': %v", window, session, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Killed window '%s' in session '%s'", window, session))
		return nil
	},
}

var nextWindowCmd = &cobra.Command{
	Use:   "next [session_name]",
	Short: "Switch to next window",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := getSessionName(args[0])
		if err := validateName(session); err != nil {
			return err
		}

		if err := manager.Backend.NextWindow(session); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to switch to next window in session '%s': %v", session, err))
			return nil
		}
		logInstance.Info("Switched to next window")
		return nil
	},
}

var prevWindowCmd = &cobra.Command{
	Use:   "prev [session_name]",
	Short: "Switch to previous window",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := getSessionName(args[0])
		if err := validateName(session); err != nil {
			return err
		}

		if err := manager.Backend.PreviousWindow(session); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to switch to previous window in session '%s': %v", session, err))
			return nil
		}
		logInstance.Info("Switched to previous window")
		return nil
	},
}

var renameWindowCmd = &cobra.Command{
	Use:   "rename [session_name] [old_name] [new_name]",
	Short: "Rename a window",
	Args:  cobra.ExactArgs(3),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := getSessionName(args[0])
		oldName := args[1]
		newName := args[2]
		if err := validateName(session); err != nil {
			return err
		}
		if err := validateName(oldName); err != nil {
			return err
		}
		if err := validateName(newName); err != nil {
			return err
		}

		if err := manager.Backend.RenameWindow(session, oldName, newName); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to rename window '%s' to '%s' in session '%s': %v", oldName, newName, session, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Renamed window '%s' to '%s' in session '%s'", oldName, newName, session))
		return nil
	},
}

var splitWindowCmd = &cobra.Command{
	Use:   "split [session_name] [window_name] [direction(v|h)]",
	Short: "Split a window into panes",
	Args:  cobra.ExactArgs(3),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := getSessionName(args[0])
		window := args[1]
		direction := args[2]
		if err := validateName(session); err != nil {
			return err
		}
		if err := validateName(window); err != nil {
			return err
		}
		if direction != "v" && direction != "h" {
			return fmt.Errorf("direction must be 'v' or 'h'")
		}

		if err := manager.Backend.SplitWindow(session, window, direction); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to split window '%s': %v", window, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Split window '%s' direction '%s'", window, direction))
		return nil
	},
}
