package cmd

import (
	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
	"github.com/spf13/cobra"
)

func init() {
	var confirm string
	c := &cobra.Command{Use: "stop ID --confirm FULL_ID", Short: "Human-confirmed stop of one exact supervised job; retain its exit receipt", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if !runtimepath.ValidID(id) || confirm != id {
			return core.Failure("invalid_usage", "agent stop requires a full managed ID and matching --confirm")
		}
		if !supervisor.Supported() {
			return core.Failure("unsupported", "managed stop requires Linux/macOS and CGO")
		}
		v, e := agent.GetExisting(cmd.Context(), core.ReadScope{Backends: []string{"native", "tmux"}, ManagedIDs: []string{id}}, id)
		if e != nil {
			return e
		}
		if selectedBackend != "" && selectedBackend != v.Locator.Backend {
			return core.Failure("invalid_target", "agent backend conflicts with --backend")
		}
		if !v.Supervised {
			return core.Failure("unsupported", "only supervised jobs have an owned process to stop")
		}
		if v.Receipt != nil {
			return agentResult(agent.Reply{Agents: []agent.View{v.View}}, nil, true)
		}
		ctx, cancel := agent.Context(cmd.Context())
		defer cancel()
		reply, e := agent.Call(ctx, agent.Request{Op: "terminate", ID: id}, true)
		return agentResult(reply, e, true)
	}}
	c.Flags().StringVar(&confirm, "confirm", "", "Independent human confirmation of the exact full managed ID")
	agentCmd.AddCommand(c)
}
