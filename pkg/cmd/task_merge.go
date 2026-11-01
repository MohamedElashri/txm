package cmd

import (
	"github.com/MohamedElashri/txm/pkg/task"
	"github.com/spf13/cobra"
)

func init() {
	var o task.MergeOptions
	c := &cobra.Command{Use: "merge ID", Short: "Review or apply a frozen fast-forward in an owned checked-out target worktree", Long: "Default --dry-run is pure. Applying requires --dry-run=false, exact OIDs/digest/full task confirmation and an explicit --quiescent assertion that all external writers and Git clients are stopped. Ref-only merge is disabled. A retained uncertain intent is never retried or rolled back. No push, cleanup or branch deletion.", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		o.ID = args[0]
		v, e := task.MergeTask(cmd.Context(), o)
		return extensionResult(v, e)
	}}
	c.Flags().StringVar(&o.Target, "target", "", "Exact local refs/heads/BRANCH target")
	c.Flags().BoolVar(&o.DryRun, "dry-run", true, "Read-only reviewed merge plan (default true)")
	c.Flags().BoolVar(&o.Quiescent, "quiescent", false, "Operator asserts all task/target writers and external Git clients are stopped")
	c.Flags().StringVar(&o.ExpectedTask, "expected-task", "", "Frozen task commit from review")
	c.Flags().StringVar(&o.ExpectedTarget, "expected-target", "", "Frozen old target commit from review")
	c.Flags().StringVar(&o.ReviewDigest, "review-digest", "", "Exact digest from merge dry-run")
	c.Flags().StringVar(&o.Confirm, "confirm", "", "Human confirmation of full task ID")
	taskCmd.AddCommand(c)
}
