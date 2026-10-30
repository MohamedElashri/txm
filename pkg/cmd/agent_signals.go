package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/supervisor"
	"github.com/spf13/cobra"
)

func credential(id, generation string) (agent.Credential, error) {
	r, e := supervisor.State()
	if e != nil {
		return agent.Credential{}, e
	}
	defer func() { _ = r.Close() }()
	return agent.ReadCredential(r, id, generation)
}

var hooksProvider string
var hooksCmd = &cobra.Command{Use: "hooks", Short: "Configure explicit provider associations"}
var hooksEnableCmd = &cobra.Command{Use: "enable ID", Short: "Create or rotate private hook credentials", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	p := agent.InspectProvider(cmd.Context(), hooksProvider)
	if !p.Supported {
		return core.Failure("unsupported_version", p.Reason)
	}
	reply, e := agent.Call(cmd.Context(), agent.Request{Op: "hooks-enable", ID: args[0], Provider: p.Name, ProviderVersion: p.Version}, true)
	return agentResult(reply, e, true)
}}
var hooksConfigCmd = &cobra.Command{Use: "config ID", Short: "Print provider settings for review; never edit provider files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	if jsonFlag {
		return core.Failure("invalid_usage", "hooks config emits a standalone settings document; omit --json")
	}
	reply, e := agent.Call(cmd.Context(), agent.Request{Op: "hooks-info", ID: args[0]}, true)
	if e != nil {
		return e
	}
	c, e := credential(args[0], reply.Agents[0].Hooks.Generation)
	if e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	data, e := agent.ProviderConfig(exe, c)
	if e != nil {
		return e
	}
	_, e = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return e
}}
var providersCmd = &cobra.Command{Use: "providers", Short: "Report installed provider versions and adapter availability", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	providers := []agent.Provider{}
	for _, name := range []string{"generic", "claude", "codex", "opencode"} {
		providers = append(providers, agent.InspectProvider(cmd.Context(), name))
	}
	if !jsonFlag {
		for _, p := range providers {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s supported=%t: %s\n", p.Name, core.Plain([]byte(p.Version)), p.Supported, p.Reason)
		}
	}
	return queryResult(struct {
		Providers []agent.Provider `json:"providers"`
	}{providers}, []*core.OperationError{}, 1)
}}
var signalFromEnv bool
var signalInstance, signalGeneration, signalTime, signalState string
var signalSequence uint64
var signalCmd = &cobra.Command{Use: "signal [ID]", Short: "Send an authenticated attention or turn signal", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	id := ""
	if len(args) > 0 {
		id = args[0]
	}
	generation := signalGeneration
	if signalFromEnv {
		if id != "" || cmd.Flags().Changed("instance") || cmd.Flags().Changed("generation") || cmd.Flags().Changed("sequence") || cmd.Flags().Changed("time") {
			return core.Failure("invalid_usage", "--from-env allocates its own association, sequence and timestamp")
		}
		id = os.Getenv("TXM_AGENT_ID")
		generation = os.Getenv("TXM_HOOK_GENERATION")
	}
	c, e := credential(id, generation)
	if e != nil {
		return e
	}
	var input agent.Signal
	if signalFromEnv {
		if os.Getenv("TXM_AGENT_INSTANCE_ID") != c.InstanceID {
			return core.Failure("denied", "hook context instance mismatch")
		}
		input, e = agent.NextSignal(cmd.Context(), c, signalState, "")
	} else {
		var timestamp time.Time
		timestamp, e = time.Parse(time.RFC3339Nano, signalTime)
		input = agent.Signal{InstanceID: signalInstance, Generation: generation, Token: c.Token, Sequence: signalSequence, Time: timestamp, State: signalState}
	}
	if e != nil {
		return core.Failure("invalid_usage", "invalid signal timestamp or cursor")
	}
	reply, e := agent.Call(cmd.Context(), agent.Request{Op: "signal", ID: id, Signal: &input}, false)
	return agentResult(reply, e, true)
}}
var hookGeneration, hookProvider string
var hookCmd = &cobra.Command{Use: "hook ID", Short: "Observational provider hook bridge (JSON stdin)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	// Always leave provider decisions unchanged. Never echo untrusted hook input.
	defer func() { _, _ = fmt.Fprintln(cmd.OutOrStdout(), "{}") }()
	run := func() error {
		if jsonFlag {
			return core.Failure("invalid_usage", "hook bridge does not use --json")
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
		defer cancel()
		c, e := credential(args[0], hookGeneration)
		if e != nil {
			return e
		}
		if c.Provider != hookProvider {
			return core.Failure("denied", "provider mismatch")
		}
		inputReader := cmd.InOrStdin()
		if closer, ok := inputReader.(io.Closer); ok {
			stop := context.AfterFunc(ctx, func() { _ = closer.Close() })
			defer stop()
		}
		data, e := io.ReadAll(io.LimitReader(inputReader, (64<<10)+1))
		if e != nil {
			return e
		}
		state, session, e := agent.ProviderInput(hookProvider, data)
		if e != nil {
			return e
		}
		if state == "" {
			return nil
		}
		input, e := agent.NextSignal(ctx, c, state, session)
		if e != nil {
			return e
		}
		_, e = agent.Call(ctx, agent.Request{Op: "signal", ID: args[0], Signal: &input}, false)
		return e
	}
	if run() != nil {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "txm: observational hook was not accepted")
	}
	return nil
}}
var waitUntil string
var waitTimeout time.Duration
var waitFail bool
var waitCmd = &cobra.Command{Use: "wait ID", Short: "Wait for an observed exit or fresh attention condition", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	if waitTimeout <= 0 || waitTimeout > 24*time.Hour {
		return core.Failure("invalid_usage", "wait timeout must be greater than zero and at most 24h")
	}
	if backendFlag != "" {
		return core.Failure("invalid_usage", "wait uses exact agent IDs; omit --backend")
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), waitTimeout)
	defer cancel()
	v, e := agent.Wait(ctx, args[0], waitUntil, waitFail)
	if jsonFlag {
		failures := []*core.OperationError{}
		if e != nil {
			failures = append(failures, core.Normalize(e))
		}
		response = &core.Envelope{SchemaVersion: 1, Data: struct {
			Agent agent.View `json:"agent"`
		}{v}, Errors: failures}
	}
	if !jsonFlag && v.ID != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s (attention: %s)\n", v.ID, v.Lifecycle, v.Attention)
	}
	return e
}}
var eventsJSONL bool
var eventsID string
var eventsTimeout time.Duration
var eventsCmd = &cobra.Command{Use: "events", Short: "Stream bounded, transcript-free agent events", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	if !eventsJSONL || jsonFlag || backendFlag != "" {
		return core.Failure("invalid_usage", "events requires --jsonl and optional --id; omit --json/--backend")
	}
	ctx := cmd.Context()
	if eventsTimeout < 0 || eventsTimeout > 24*time.Hour {
		return core.Failure("invalid_usage", "events timeout must be zero or at most 24h")
	}
	if eventsTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, eventsTimeout)
		defer cancel()
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return agent.Stream(ctx, eventsID, func(e agent.Event) error { return encoder.Encode(e) })
}}
var notifyCmd = &cobra.Command{Use: "notify -- EXECUTABLE [ARGS...]", Short: "Explicitly run a bounded action for future attention and exit events", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	if cmd.ArgsLenAtDash() != 0 || jsonFlag || backendFlag != "" {
		return core.Failure("invalid_usage", "notify requires -- EXECUTABLE [ARGS...], without --json/--backend")
	}
	exe, e := exec.LookPath(args[0])
	if e != nil {
		return core.Failure("invalid_usage", "notification action unavailable")
	}
	return agent.Notifications(cmd.Context(), func(ctx context.Context, payload []byte) error {
		child := exec.CommandContext(ctx, exe, args[1:]...)
		child.WaitDelay = 250 * time.Millisecond
		child.Stdin = bytes.NewReader(payload)
		child.Stdout = io.Discard
		child.Stderr = io.Discard
		return child.Run()
	})
}}

func init() {
	hooksEnableCmd.Flags().StringVar(&hooksProvider, "provider", "generic", "Provider: generic, claude or codex (verified versions only)")
	hooksCmd.AddCommand(hooksEnableCmd, hooksConfigCmd)
	signalCmd.Flags().BoolVar(&signalFromEnv, "from-env", false, "Use the explicit managed hook context and allocate a sequence")
	signalCmd.Flags().StringVar(&signalInstance, "instance", "", "Associated instance ID")
	signalCmd.Flags().StringVar(&signalGeneration, "generation", "", "Credential generation")
	signalCmd.Flags().Uint64Var(&signalSequence, "sequence", 0, "Strictly increasing sequence")
	signalCmd.Flags().StringVar(&signalTime, "time", "", "RFC3339 timestamp")
	signalCmd.Flags().StringVar(&signalState, "state", "", "unknown, working, needs-input, idle or turn-complete")
	hookCmd.Flags().StringVar(&hookGeneration, "generation", "", "Private credential generation")
	hookCmd.Flags().StringVar(&hookProvider, "provider", "", "Verified provider adapter")
	waitCmd.Flags().StringVar(&waitUntil, "until", "exited", "exited, needs-input or idle")
	waitCmd.Flags().DurationVar(&waitTimeout, "timeout", 30*time.Second, "Finite wait deadline (maximum 24h)")
	waitCmd.Flags().BoolVar(&waitFail, "fail-on-nonzero", false, "Fail when the observed child exit was unsuccessful")
	eventsCmd.Flags().BoolVar(&eventsJSONL, "jsonl", false, "Emit one versioned event per line")
	eventsCmd.Flags().StringVar(&eventsID, "id", "", "Filter by exact agent ID")
	eventsCmd.Flags().DurationVar(&eventsTimeout, "timeout", 0, "Optional stream deadline")
	agentCmd.AddCommand(hooksCmd, providersCmd, signalCmd, hookCmd, waitCmd, notifyCmd)
	rootCmd.AddCommand(eventsCmd)
}
