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
	Use:               "list [session_name] [window_name]",
	Aliases:           []string{"ls"},
	Short:             "List panes in a window",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, window, err := parseWindowArgs(args)
		if err != nil {
			return err
		}

		panes, err := manager.Backend.Panes(cmd.Context(), session, window)
		if err != nil {
			return err
		}
		for _, p := range panes {
			fmt.Printf("%d: %s\n", p.Index, p.ID)
		}
		return nil
	},
}

var paneKillCmd = &cobra.Command{
	Use:               "kill [session_name] [window_name] [pane_id]",
	Aliases:           []string{"delete", "rm"},
	Short:             "Remove a pane",
	Args:              cobra.ExactArgs(3),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, window, pane, err := parsePaneArgs(args)
		if err != nil {
			return err
		}

		if err := manager.Backend.KillPane(session, window, pane); err != nil {
			return fmt.Errorf("failed to kill pane '%s' in window '%s': %w", pane, window, err)
		}
		logInstance.Info(fmt.Sprintf("Killed pane '%s' in window '%s'", pane, window))
		return nil
	},
}

var paneExecCmd = &cobra.Command{
	Use:               "exec [session_name] [window_name] [pane_id] [command]",
	Short:             "Send literal terminal input to an exact pane",
	Args:              cobra.ExactArgs(4),
	ValidArgsFunction: getSingleSessionCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, window, pane, err := parsePaneArgs(args)
		if err != nil {
			return err
		}
		command := args[3]

		if err := manager.Backend.Exec(session, window, pane, command); err != nil {
			return fmt.Errorf("failed to execute command in pane '%s': %w", pane, err)
		}
		logInstance.Info(fmt.Sprintf("Sent input to pane '%s'", pane))
		return nil
	},
}

func init() {
	paneCmd.AddCommand(paneListCmd)
	paneCmd.AddCommand(paneKillCmd)
	paneCmd.AddCommand(paneExecCmd)
}
