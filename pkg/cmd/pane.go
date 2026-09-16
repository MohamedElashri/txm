package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var paneCmd = &cobra.Command{
	Use:     "pane",
	Aliases: []string{"p"},
	Short:   "Manage panes within a window",
}

var paneListCmd = &cobra.Command{
	Use:     "list [session_name] [window_name]",
	Aliases: []string{"ls"},
	Short:   "List panes in a window",
	Args:    cobra.ExactArgs(2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, window, err := parseWindowArgs(args)
		if err != nil {
			return err
		}

		if err := manager.Backend.ListPanes(session, window); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to list panes in window '%s': %v", window, err))
		}
		return nil
	},
}

var paneKillCmd = &cobra.Command{
	Use:     "kill [session_name] [window_name] [pane_id]",
	Aliases: []string{"delete", "rm"},
	Short:   "Remove a pane",
	Args:    cobra.ExactArgs(3),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, window, pane, err := parsePaneArgs(args)
		if err != nil {
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

var paneExecCmd = &cobra.Command{
	Use:   "exec [session_name] [window_name] [pane_id] [command]",
	Short: "Execute a command in a pane",
	Args:  cobra.ExactArgs(4),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, window, pane, err := parsePaneArgs(args)
		if err != nil {
			return err
		}
		command := args[3]

		if err := manager.Backend.Exec(session, window, pane, command); err != nil {
			logInstance.Error(fmt.Sprintf("Failed to execute command in pane '%s': %v", pane, err))
			return nil
		}
		logInstance.Info(fmt.Sprintf("Executed command in pane '%s'", pane))
		return nil
	},
}

func init() {
	paneCmd.AddCommand(paneListCmd)
	paneCmd.AddCommand(paneKillCmd)
	paneCmd.AddCommand(paneExecCmd)
}
