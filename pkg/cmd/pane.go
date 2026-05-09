package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"strconv"
)

var listPanesCmd = &cobra.Command{
	Use:   "list-panes [session_name] [window_name]",
	Short: "List panes in a window",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
		window := args[1]
		if err := validateName(session); err != nil {
			return err
		}
		if err := validateName(window); err != nil {
			return err
		}

		if err := manager.Backend.ListPanes(session, window); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to list panes in window '%s': %v", window, err))
		}
		return nil
	},
}

var killPaneCmd = &cobra.Command{
	Use:   "kill-pane [session_name] [window_name] [pane_id]",
	Short: "Remove a pane",
	Args:  cobra.ExactArgs(3),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
		window := args[1]
		pane := args[2]
		if err := validateName(session); err != nil {
			return err
		}
		if err := validateName(window); err != nil {
			return err
		}

		if err := manager.Backend.KillPane(session, window, pane); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to kill pane '%s' in window '%s': %v", pane, window, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Killed pane '%s' in window '%s'", pane, window))
		return nil
	},
}

var resizePaneCmd = &cobra.Command{
	Use:   "resize-pane [session_name] [window_name] [pane_id] [direction(U|D|L|R)] [size]",
	Short: "Resize a pane",
	Args:  cobra.ExactArgs(5),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
		window := args[1]
		pane := args[2]
		direction := args[3]
		sizeStr := args[4]

		if err := validateName(session); err != nil {
			return err
		}
		if err := validateName(window); err != nil {
			return err
		}
		if direction != "U" && direction != "D" && direction != "L" && direction != "R" {
			return fmt.Errorf("direction must be U, D, L, or R")
		}

		size, err := strconv.Atoi(sizeStr)
		if err != nil || size <= 0 {
			return fmt.Errorf("size must be a positive integer")
		}

		if err := manager.Backend.ResizePane(session, window, pane, direction, size); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to resize pane '%s' in window '%s': %v", pane, window, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Resized pane '%s' in window '%s' by %d %s", pane, window, size, direction))
		return nil
	},
}

var sendKeysCmd = &cobra.Command{
	Use:   "send-keys [session_name] [window_name] [pane_id] [keys]",
	Short: "Send keystrokes to a pane",
	Args:  cobra.ExactArgs(4),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := args[0]
		window := args[1]
		pane := args[2]
		keys := args[3]

		if err := validateName(session); err != nil {
			return err
		}
		if err := validateName(window); err != nil {
			return err
		}

		if err := manager.Backend.SendKeys(session, window, pane, keys); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to send keys to pane '%s': %v", pane, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Sent keys to pane '%s'", pane))
		return nil
	},
}
