package cmd

import (
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/task"
	"github.com/spf13/cobra"
)

func init() {
	taskCmd.AddCommand(&cobra.Command{Use: "setup-status ID", Short: "Read retained setup intent and receipt without starting services", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		v, e := task.SetupStatus(cmd.Context(), args[0])
		return extensionResult(v, e)
	}})
	var target string
	var diff bool
	c := &cobra.Command{Use: "review ID", Short: "Read an owned task's frozen base/head/target comparison without starting services", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if selectedBackend != "" && selectedBackend != "native" && selectedBackend != "tmux" {
			return core.Failure("unsupported", "task review requires native/tmux provenance")
		}
		v, err := task.ReviewTask(cmd.Context(), args[0], target, diff)
		return extensionResult(v, err)
	}}
	c.Flags().StringVar(&target, "target", "", "Exact local target refs/heads/BRANCH (required)")
	c.Flags().BoolVar(&diff, "diff", false, "Explicitly include up to 16 KiB of untrusted diff; terminal controls stripped, secrets not redacted")
	taskCmd.AddCommand(c)
}
