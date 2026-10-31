package cmd

import (
	"fmt"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/spf13/cobra"
)

var doctorProbe bool
var doctorCmd = &cobra.Command{
	Use: "doctor", Short: "Discover providers without starting services or changing trust",
	Long: "Report provider availability, schema compatibility and validation separately. Default discovery only inspects PATH. --probe runs bounded provider version commands; it never enables hooks, changes trust or starts the coordinator.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		r := agent.Doctor(cmd.Context(), doctorProbe)
		if !jsonFlag {
			for _, p := range r.Providers {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s available=%t validation=%s: %s\n", p.Name, p.Available, p.Validation, core.Plain([]byte(p.Reason)))
			}
			for _, guidance := range r.Guidance {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), guidance)
			}
		}
		return queryResult(r, []*core.OperationError{}, 1)
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorProbe, "probe", false, "Execute bounded installed-provider version commands (may create provider caches)")
	agentCmd.AddCommand(doctorCmd)
}
