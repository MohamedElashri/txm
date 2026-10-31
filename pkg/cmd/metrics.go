package cmd

import (
	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/metrics"
	"github.com/spf13/cobra"
)

func init() {
	agentCmd.AddCommand(&cobra.Command{Use: "metrics ID", Short: "Read existing local child-PID CPU/RSS samples; never start a sampler or coordinator", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		backends := []string{"native", "tmux"}
		if selectedBackend != "" {
			backends = []string{selectedBackend}
		}
		v, e := agent.GetExisting(cmd.Context(), core.ReadScope{Backends: backends, ManagedIDs: []string{args[0]}}, args[0])
		if e != nil {
			return e
		}
		if !v.Supervised || v.InstanceID == nil {
			return core.Failure("unsupported", "metrics require a local supervised child")
		}
		out, e := metrics.Read(cmd.Context(), metrics.Owner{ManagedID: v.ID, InstanceID: *v.InstanceID, Backend: v.Locator.Backend})
		return extensionResult(out, e)
	}})
}
