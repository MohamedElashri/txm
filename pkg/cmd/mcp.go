package cmd

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/control"
	"github.com/MohamedElashri/txm/pkg/core"
	localmcp "github.com/MohamedElashri/txm/pkg/mcp"
	"github.com/MohamedElashri/txm/pkg/supervisor"
	"github.com/spf13/cobra"
	"io"
)

var mcpReadIDs, mcpReadRepos, mcpReadBackends []string
var mcpReadHost string
var mcpCreatePolicy string
var mcpCmd = &cobra.Command{Use: "mcp", Short: "Explicitly scoped local MCP over stdio"}
var mcpServeCmd = &cobra.Command{
	Use: "serve", Short: "Serve bounded scoped tools over local stdio",
	Long: "Serve MCP 2025-11-25 initialization or 2026-07-28 per-request metadata over newline-delimited stdio. Reads default to deny. Supply read backend plus exact managed IDs and/or owned repository roots; selectors intersect. Session tools report managed registrations, not backend liveness or attachment proof. Creates require --create-policy with an owned mode-0600 JSON template grant; default catalog has no writes. The initial template environment is empty. No preview, remote discovery or destructive tools. Limits: 1 MiB messages, 16 concurrent requests, four waits, 60-second maximum wait, 8192 unique request IDs per process; reconnect before exhausting the ID budget.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if !supervisor.Supported() {
			return core.Failure("unsupported", "scoped MCP state requires Linux/macOS and CGO")
		}
		if jsonFlag {
			return core.Failure("invalid_usage", "MCP owns stdout; omit --json")
		}
		backends := mcpReadBackends
		if selectedBackend != "" {
			for _, b := range backends {
				if b != selectedBackend {
					return core.Failure("invalid_usage", "read backend conflicts with --backend")
				}
			}
			if len(backends) == 0 {
				backends = []string{selectedBackend}
			}
		}
		s, err := localmcp.New(core.ReadScope{HostID: mcpReadHost, Backends: backends, ManagedIDs: mcpReadIDs, Repositories: mcpReadRepos}, Version)
		if err != nil {
			return err
		}
		if mcpCreatePolicy != "" {
			creator, err := control.LoadCreator(mcpCreatePolicy)
			if err != nil {
				return err
			}
			s.EnableCreates(creator)
		}
		// Cobra's streams are the process stdio streams here. Never route the
		// CLI result envelope or informational logger through this transport.
		return runMCP(cmd, s)
	},
}

type mcpWriter struct{ io.Writer }

func (mcpWriter) Close() error { return nil }

func runMCP(cmd *cobra.Command, s *localmcp.Server) error {
	input, ok := cmd.InOrStdin().(io.ReadCloser)
	if !ok {
		input = io.NopCloser(cmd.InOrStdin())
	}
	output, ok := cmd.OutOrStdout().(io.WriteCloser)
	if !ok {
		output = mcpWriter{cmd.OutOrStdout()}
	}
	if err := s.Run(cmd.Context(), input, output); err != nil {
		if cmd.Context().Err() != nil {
			return cmd.Context().Err()
		}
		if err == context.Canceled {
			return err
		}
		return core.Failure("protocol_error", "MCP stdio server stopped after a protocol or transport error")
	}
	return nil
}

func init() {
	mcpServeCmd.Flags().StringVar(&mcpCreatePolicy, "create-policy", "", "Explicit owned mode-0600 JSON template grant; child environment is empty")
	mcpServeCmd.Flags().StringArrayVar(&mcpReadIDs, "read-id", nil, "Allow an exact managed ID (repeatable)")
	mcpServeCmd.Flags().StringArrayVar(&mcpReadRepos, "read-repo", nil, "Allow an owned canonical repository root; intersects ID selectors")
	mcpServeCmd.Flags().StringArrayVar(&mcpReadBackends, "read-backend", nil, "Allow native or tmux reads (repeatable); no implicit backend")
	mcpServeCmd.Flags().StringVar(&mcpReadHost, "read-host", "", "Require this local host ID; never discover SSH hosts")
	mcpCmd.AddCommand(mcpServeCmd)
	rootCmd.AddCommand(mcpCmd)
}
