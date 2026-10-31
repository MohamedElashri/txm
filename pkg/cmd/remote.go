package cmd

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/remote"
	"github.com/spf13/cobra"
	"time"
)

var remoteCmd = &cobra.Command{Use: "remote", Short: "Opt-in pinned SSH aggregation"}
var remoteExecutable, remoteHostID, remoteRequest string
var remoteTimeout time.Duration
var attachRequest remote.Request
var remoteEndpointCmd = &cobra.Command{Use: "remote", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	q, e := remote.Decode(remoteRequest)
	if e != nil {
		jsonFlag = true
		return e
	}
	if q.Op == "attach" {
		if jsonFlag {
			return core.Failure("invalid_usage", "interactive attach does not support JSON")
		}
		_, e = remote.Execute(cmd.Context(), q)
		return e
	}
	jsonFlag = true
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()
	d, e := remote.Execute(ctx, q)
	if e != nil {
		return e
	}
	return queryResult(d, []*core.OperationError{}, 1)
}}
var remoteListCmd = &cobra.Command{Use: "list [ALIAS...]", Short: "List pinned hosts with bounded requests and partial failures", RunE: func(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
	defer cancel()
	hosts, e := remote.Hosts(ctx)
	if e != nil {
		return e
	}
	b := selectedBackend
	if b == "" {
		b = "native"
	}
	results, e := remote.List(ctx, hosts, args, b, remoteTimeout)
	if e != nil {
		return e
	}
	failures := []*core.OperationError{}
	successes := 0
	for _, r := range results {
		if r.Error != nil {
			f := *r.Error
			f.Message = r.Alias + ": " + f.Message
			f.Locator = &core.SessionLocator{HostID: r.HostID, Backend: b, Name: r.Alias}
			failures = append(failures, &f)
		} else {
			successes++
		}
	}
	if !jsonFlag {
		_ = extensionResult(results, nil)
	}
	return queryResult(results, failures, successes)
}}
var remoteProbeCmd = &cobra.Command{Use: "probe ALIAS", Short: "Negotiate protocol and inspect remote host identity", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()
	hs, e := remote.Hosts(ctx)
	if e != nil {
		return e
	}
	h, e := remote.Select(hs, args[0])
	if e != nil {
		return e
	}
	d, e := remote.Query(ctx, h, remote.Request{Schema: 1, Protocol: 1, Op: "probe", HostID: h.HostID}, remoteTimeout)
	if e != nil {
		return e
	}
	return extensionResult(d, nil)
}}
var remoteAttachCmd = &cobra.Command{Use: "attach ALIAS", Short: "Attach to an exact observed session on a pinned SSH host", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	if jsonFlag {
		return core.Failure("invalid_usage", "interactive attach does not support JSON")
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	hs, e := remote.Hosts(ctx)
	cancel()
	if e != nil {
		return e
	}
	h, e := remote.Select(hs, args[0])
	if e != nil {
		return e
	}
	q := attachRequest
	q.Schema = 1
	q.Protocol = 1
	q.Op = "attach"
	q.Backend = selectedBackend
	if q.Backend == "" {
		return core.Failure("invalid_usage", "remote attach requires explicit --backend")
	}
	return remote.Attach(cmd.Context(), h, q)
}}

func init() {
	rootCmd.AddCommand(remoteCmd)
	remoteCmd.AddCommand(remoteListCmd, remoteProbeCmd, remoteAttachCmd)
	internalCmd.AddCommand(remoteEndpointCmd)
	remoteEndpointCmd.Flags().StringVar(&remoteRequest, "request", "", "Encoded internal protocol request")
	remoteListCmd.Flags().DurationVar(&remoteTimeout, "timeout", 10*time.Second, "Per-host timeout (1..30 seconds)")
	remoteProbeCmd.Flags().DurationVar(&remoteTimeout, "timeout", 10*time.Second, "Per-host timeout (1..30 seconds)")
	remoteAttachCmd.Flags().StringVar(&attachRequest.HostID, "host-id", "", "Exact configured host identity (required)")
	remoteAttachCmd.Flags().StringVar(&attachRequest.Name, "name", "", "Exact observed session name (required)")
	remoteAttachCmd.Flags().StringVar(&attachRequest.BackendID, "session-id", "", "Exact backend session ID (required)")
	remoteAttachCmd.Flags().StringVar(&attachRequest.Instance, "instance", "", "Exact native launch instance")
	remoteAttachCmd.Flags().BoolVar(&attachRequest.ReadOnly, "read-only", false, "Attach as read-only observer")
	allow := &cobra.Command{Use: "allow ALIAS", Short: "Explicitly allow one SSH configuration alias", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		v, e := remote.Allow(ctx, remote.Host{Alias: args[0], HostID: remoteHostID, Executable: remoteExecutable})
		if e != nil {
			return e
		}
		return extensionResult(v, nil)
	}}
	allow.Flags().StringVar(&remoteExecutable, "executable", "txm", "Remote executable: txm or a safe absolute path token")
	allow.Flags().StringVar(&remoteHostID, "host-id", "", "Explicitly pin a verified host identity")
	remoteCmd.AddCommand(allow)
	remoteCmd.AddCommand(&cobra.Command{Use: "hosts", Short: "Read the explicit SSH allowlist", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		v, e := remote.Hosts(ctx)
		if e != nil {
			return e
		}
		return extensionResult(v, nil)
	}})
	remoteCmd.AddCommand(&cobra.Command{Use: "pin ALIAS HOST_ID", Short: "Explicitly pin a verified host identity", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		v, e := remote.Pin(ctx, args[0], args[1])
		if e != nil {
			return e
		}
		return extensionResult(v, nil)
	}})
	remoteCmd.AddCommand(&cobra.Command{Use: "forget ALIAS", Short: "Remove an SSH allowlist entry", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		v, e := remote.Forget(ctx, args[0])
		if e != nil {
			return e
		}
		return extensionResult(v, nil)
	}})
}
