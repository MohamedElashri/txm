package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var newWindowCmd = &cobra.Command{
	Use:   "new-window [session_name] [window_name]",
	Short: "Create a new window/tab",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
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
	Use:   "list-windows [session_name]",
	Short: "List windows in a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
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
	Use:   "kill-window [session_name] [window_name]",
	Short: "Remove a window",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
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
	Use:   "next-window [session_name]",
	Short: "Switch to next window",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
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
	Use:   "prev-window [session_name]",
	Short: "Switch to previous window",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
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
	Use:   "rename-window [session_name] [old_name] [new_name]",
	Short: "Rename a window",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
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

var moveWindowCmd = &cobra.Command{
	Use:   "move-window [src_session] [window_name] [dst_session]",
	Short: "Move window between sessions",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		srcSession := args[0]
		windowName := args[1]
		dstSession := args[2]
		if err := validateName(srcSession); err != nil {
			return err
		}
		if err := validateName(windowName); err != nil {
			return err
		}
		if err := validateName(dstSession); err != nil {
			return err
		}

		if err := manager.Backend.MoveWindow(srcSession, windowName, dstSession); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to move window '%s' to session '%s': %v", windowName, dstSession, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Moved window '%s' to session '%s'", windowName, dstSession))
		return nil
	},
}

var swapWindowCmd = &cobra.Command{
	Use:   "swap-window [session_name] [window1] [window2]",
	Short: "Swap window positions",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
		window1 := args[1]
		window2 := args[2]
		if err := validateName(session); err != nil {
			return err
		}
		if err := validateName(window1); err != nil {
			return err
		}
		if err := validateName(window2); err != nil {
			return err
		}

		if err := manager.Backend.SwapWindow(session, window1, window2); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to swap windows '%s' and '%s': %v", window1, window2, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Swapped windows '%s' and '%s'", window1, window2))
		return nil
	},
}

var splitWindowCmd = &cobra.Command{
	Use:   "split-window [session_name] [window_name] [direction(v|h)]",
	Short: "Split a window into panes",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
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
