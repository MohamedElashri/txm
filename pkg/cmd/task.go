package cmd

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/task"
	"github.com/spf13/cobra"
	"time"
)

var taskCmd = &cobra.Command{Use: "task", Short: "Opt-in Git worktree tasks backed by managed agents"}
var taskOptions task.Options
var taskCleanupDry, taskForce bool
var taskConfirm string
var taskStartCmd = &cobra.Command{Use: "start NAME -- EXECUTABLE [ARGS...]", Short: "Create a fresh worktree and supervised command", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
	if cmd.ArgsLenAtDash() != 1 {
		return core.Failure("invalid_usage", "task start requires NAME -- EXECUTABLE [ARGS...]")
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
	defer cancel()
	o := taskOptions
	o.Name = args[0]
	o.Backend = selectedBackend
	if o.Backend == "" {
		o.Backend = "native"
	}
	exe, e := extension.Resolve(args[1])
	if e != nil {
		return e
	}
	o.Argv = append([]string{exe}, args[2:]...)
	names := []string{"HOME", "PATH", "TERM"}
	seen := map[string]bool{"HOME": true, "PATH": true, "TERM": true}
	for _, n := range o.Env {
		if !seen[n] {
			names = append(names, n)
			seen[n] = true
		}
	}
	o.Env = names
	v, e := task.Start(ctx, o)
	return extensionResult(v, e)
}}
var taskListCmd = &cobra.Command{Use: "list", Short: "List tasks and existing managed lifecycle state", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()
	v, e := task.List(ctx)
	if e != nil {
		return e
	}
	failures := []*core.OperationError{}
	for _, t := range v {
		if t.Error != nil {
			failures = append(failures, t.Error)
		}
	}
	if len(failures) > 0 {
		if !jsonFlag {
			_ = extensionResult(v, nil)
		}
		return queryResult(v, failures, len(v))
	}
	return extensionResult(v, nil)
}}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskStartCmd, taskListCmd)
	taskStartCmd.Flags().StringVar(&taskOptions.SetupPolicy, "setup-policy", "", "Owned mode-0600 JSON grant binding exact task/setup argv and worktree; review --dry-run first")
	taskStartCmd.Flags().StringVar(&taskOptions.AcceptSetup, "accept-setup", "", "Exact reviewed dry-run setup digest; changed inputs invalidate it")
	taskStartCmd.Flags().StringVar(&taskOptions.Repo, "repo", "", "Source Git repository (required)")
	taskStartCmd.Flags().StringVar(&taskOptions.Base, "base", "HEAD", "Committed starting revision")
	taskStartCmd.Flags().StringVar(&taskOptions.Branch, "branch", "", "Fresh branch (default txm/NAME-ID)")
	taskStartCmd.Flags().StringArrayVar(&taskOptions.Env, "env", nil, "Allow environment variable name (repeatable)")
	taskStartCmd.Flags().BoolVar(&taskOptions.DryRun, "dry-run", false, "Read-only preflight; redact command/environment values")
	taskStartCmd.Flags().BoolVar(&taskOptions.AllowRepoCommands, "allow-repo-commands", false, "Explicitly permit repository Git hooks/filters")
	for _, operation := range []string{"inspect", "stop", "cleanup"} {
		op := operation
		c := &cobra.Command{Use: op + " ID", Short: op + " one exact task ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			var v task.View
			var e error
			if selectedBackend != "" {
				observed, err := task.Inspect(ctx, args[0])
				if err != nil {
					return err
				}
				if observed.Task.Backend != selectedBackend {
					return core.Failure("invalid_target", "task backend conflicts with --backend")
				}
			}
			switch op {
			case "inspect":
				v, e = task.Inspect(ctx, args[0])
			case "stop":
				v, e = task.Stop(ctx, args[0])
			case "cleanup":
				v, e = task.Cleanup(ctx, args[0], taskCleanupDry, taskForce, taskConfirm)
			}
			if v.Task.ID == "" {
				return e
			}
			return extensionResult(v, e)
		}}
		if op == "cleanup" {
			c.Flags().BoolVar(&taskCleanupDry, "dry-run", false, "Verify cleanup without Git removal")
			c.Flags().BoolVar(&taskForce, "force", false, "Permit dirty worktree removal with exact confirmation")
			c.Flags().StringVar(&taskConfirm, "confirm", "", "Confirm full task ID for force cleanup")
		}
		taskCmd.AddCommand(c)
	}
}
