package cmd

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/dashboard"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var dashboardAgents, dashboardPreview bool
var dashboardRefresh time.Duration
var dashboardCmd = &cobra.Command{Use: "dashboard", Short: "Interactive local session observations and optional agent state", Long: "Persistent terminals. Supervised jobs. One CLI. Plain dashboard does not start an agent coordinator. --agents explicitly subscribes. Fresh authenticated needs-input agents sort first; permission requests remain unconfirmed. [] next/previous blocked; g repo/task/backend grouping; c collapse group; C expand all; / search; Tab filter; b backend; o read-only attach; p bounded preview; F1 help. Registrations and sessions keep separate identities; attach requires a live session binding.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return core.Failure("unsupported", "dashboard terminal runtime is unverified on this platform")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return core.Failure("invalid_usage", "dashboard requires terminal stdin and stdout; use list/status --json for scripts")
	}
	if dashboardRefresh < 500*time.Millisecond || dashboardRefresh > time.Minute {
		return core.Failure("invalid_usage", "dashboard refresh must be 500ms..1m")
	}
	id, e := hostID(cmd.Context())
	if e != nil {
		return e
	}
	names := backend.BackendNames()
	if selectedBackend != "" {
		names = []string{selectedBackend}
	}
	return dashboard.Run(cmd.Context(), dashboard.Options{CurrentNativeID: dashboard.CurrentNativeInstance(), HostID: id, Backends: names, Agents: dashboardAgents, Preview: dashboardPreview, Refresh: dashboardRefresh}, dashboardSource{}, nil)
}}

type dashboardSource struct{}

func (dashboardSource) Sessions(ctx context.Context, name string) ([]core.Session, error) {
	b, e := backend.NewBackend(name, ctx)
	if e != nil {
		return nil, e
	}
	return b.Sessions(ctx)
}
func (dashboardSource) WatchAgents(ctx context.Context, receive func([]agent.View)) error {
	return dashboard.WatchAgentViews(ctx, receive)
}
func (dashboardSource) Preview(ctx context.Context, s core.Session, o core.CaptureOptions) (core.Snapshot, error) {
	b, e := backend.NewBackend(s.Locator.Backend, ctx)
	if e != nil {
		return core.Snapshot{}, e
	}
	return backend.CaptureObserved(ctx, b, s, o)
}
func (dashboardSource) Attach(ctx context.Context, s core.Session, readOnly bool) error {
	b, e := backend.NewBackend(s.Locator.Backend, ctx)
	if e != nil {
		return e
	}
	return backend.AttachObserved(ctx, b, s, readOnly)
}
func init() {
	rootCmd.AddCommand(dashboardCmd)
	dashboardCmd.Flags().BoolVar(&dashboardAgents, "agents", false, "Explicitly subscribe to agent state (may start coordinator)")
	dashboardCmd.Flags().BoolVar(&dashboardPreview, "preview", false, "Capture bounded plain output for the selected session")
	dashboardCmd.Flags().DurationVar(&dashboardRefresh, "refresh", 2*time.Second, "Backend refresh interval (500ms..1m)")
}
