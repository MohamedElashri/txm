package cmd

import (
	"fmt"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/spf13/cobra"
)

var windowCmd = &cobra.Command{
	Use:     "window",
	Aliases: []string{"w"},
	Short:   "Manage windows within a session",
}

var windowCreateCmd = &cobra.Command{
	Use:               "new [session_name] [window_name]",
	Aliases:           []string{"create", "c"},
	Short:             "Create a new window/tab",
	Args:              cobra.RangeArgs(1, 2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := parseSessionArgs(args)
		if err != nil {
			return err
		}
		var windowName string
		if len(args) > 1 {
			windowName = args[1]
		}

		if windowName != "" {
			if err := validateName(windowName); err != nil {
				return err
			}
		}
		if err := manager.Backend.NewWindow(session, windowName); err != nil {
			return fmt.Errorf("failed to create window in session '%s': %w", session, err)
		}
		logInstance.Info(fmt.Sprintf("Created window in session '%s'", session))
		return nil
	},
}

var windowListCmd = &cobra.Command{
	Use:               "list [session_name]",
	Aliases:           []string{"ls"},
	Short:             "List windows in a session",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := parseSessionArgs(args)
		if err != nil {
			return err
		}

		windows, err := manager.Backend.Windows(cmd.Context(), session)
		if err != nil {
			return err
		}
		for _, w := range windows {
			fmt.Printf("%d: %s (%s)\n", w.Index, core.Plain([]byte(w.Name)), w.ID)
		}
		return nil
	},
}

var windowKillCmd = &cobra.Command{
	Use:               "kill [session_name] [window_name]",
	Aliases:           []string{"delete", "rm"},
	Short:             "Remove a window",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, window, err := parseWindowArgs(args)
		if err != nil {
			return err
		}

		if err := manager.Backend.KillWindow(session, window); err != nil {
			return fmt.Errorf("failed to kill window '%s' in session '%s': %w", window, session, err)
		}
		logInstance.Info(fmt.Sprintf("Killed window '%s' in session '%s'", window, session))
		return nil
	},
}

var windowNextCmd = &cobra.Command{
	Use:               "next [session_name]",
	Short:             "Switch to next window",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := parseSessionArgs(args)
		if err != nil {
			return err
		}

		if err := manager.Backend.NextWindow(session); err != nil {
			return fmt.Errorf("failed to switch to next window in session '%s': %w", session, err)
		}
		logInstance.Info("Switched to next window")
		return nil
	},
}

var windowPrevCmd = &cobra.Command{
	Use:               "prev [session_name]",
	Short:             "Switch to previous window",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := parseSessionArgs(args)
		if err != nil {
			return err
		}

		if err := manager.Backend.PreviousWindow(session); err != nil {
			return fmt.Errorf("failed to switch to previous window in session '%s': %w", session, err)
		}
		logInstance.Info("Switched to previous window")
		return nil
	},
}

var windowRenameCmd = &cobra.Command{
	Use:               "rename [session_name] [old_name] [new_name]",
	Aliases:           []string{"mv"},
	Short:             "Rename a window",
	Args:              cobra.ExactArgs(3),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, oldName, err := parseWindowArgs(args)
		if err != nil {
			return err
		}
		newName := args[2]
		if err := validateName(newName); err != nil {
			return err
		}

		if err := manager.Backend.RenameWindow(session, oldName, newName); err != nil {
			return fmt.Errorf("failed to rename window '%s' to '%s' in session '%s': %w", oldName, newName, session, err)
		}
		logInstance.Info(fmt.Sprintf("Renamed window '%s' to '%s' in session '%s'", oldName, newName, session))
		return nil
	},
}

var windowSplitCmd = &cobra.Command{
	Use:               "split [session_name] [window_name] [direction(v|h)]",
	Short:             "Split a window into panes",
	Args:              cobra.ExactArgs(3),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, window, err := parseWindowArgs(args)
		if err != nil {
			return err
		}
		direction := args[2]
		if direction != "v" && direction != "h" {
			return core.Failure("invalid_usage", "direction must be v or h")
		}

		if err := manager.Backend.SplitWindow(session, window, direction); err != nil {
			return fmt.Errorf("failed to split window '%s': %w", window, err)
		}
		logInstance.Info(fmt.Sprintf("Split window '%s' direction '%s'", window, direction))
		return nil
	},
}

func init() {
	windowCmd.AddCommand(windowCreateCmd)
	windowCmd.AddCommand(windowListCmd)
	windowCmd.AddCommand(windowKillCmd)
	windowCmd.AddCommand(windowNextCmd)
	windowCmd.AddCommand(windowPrevCmd)
	windowCmd.AddCommand(windowRenameCmd)
	windowCmd.AddCommand(windowSplitCmd)
}
