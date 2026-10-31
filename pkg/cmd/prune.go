package cmd

import (
	"context"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/task"
	"github.com/spf13/cobra"
)

func init() {
	for _, scope := range []string{"task", "agent"} {
		kind := scope
		var dry bool
		var confirm string
		c := &cobra.Command{Use: "prune ID", Short: "Remove one exact metadata record; retain receipts and terminal/Git resources", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if !runtimepath.ValidID(id) || (!dry && confirm != id) || (confirm != "" && confirm != id) {
				return core.Failure("invalid_usage", "prune requires full ID and --confirm ID, or --dry-run")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if kind == "agent" {
				r, e := agent.Call(ctx, agent.Request{Op: "prune", ID: id, Backend: selectedBackend, DryRun: dry, Confirm: confirm}, true)
				return agentResult(r, e, true)
			}
			if selectedBackend != "" {
				v, e := task.Inspect(ctx, id)
				if e != nil {
					return e
				}
				if v.Task.Backend != selectedBackend {
					return core.Failure("invalid_target", "task backend conflicts with --backend")
				}
			}
			v, e := task.Prune(ctx, id, dry, confirm)
			if v.Task.ID == "" {
				return e
			}
			return extensionResult(v, e)
		}}
		c.Flags().BoolVar(&dry, "dry-run", false, "Verify without removing metadata")
		c.Flags().StringVar(&confirm, "confirm", "", "Confirm full opaque ID")
		if kind == "task" {
			taskCmd.AddCommand(c)
		} else {
			agentCmd.AddCommand(c)
		}
	}
}
