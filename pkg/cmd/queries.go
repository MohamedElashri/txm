package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/MohamedElashri/txm/pkg/core"
	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/spf13/cobra"
)

var backendFlag, selectedBackend string
var allBackends, jsonFlag bool
var response *core.Envelope
var outputLines = 100
var outputBytes = core.MaxBytes
var outputPlain, outputRaw, outputHistory bool

type queryFailure struct {
	failures []*core.OperationError
	partial  bool
}

func (e *queryFailure) Error() string {
	if len(e.failures) == 0 {
		return "query failed"
	}
	return e.failures[0].Message
}
func isRead(cmd *cobra.Command) bool {
	return cmd == sessionListCmd || cmd == listCmd || cmd == sessionInspectCmd || cmd == sessionOutputCmd || cmd == backendCapabilitiesCmd || cmd == sessionDumpCmd || cmd == dumpCmd
}
func aggregateRead(cmd *cobra.Command) bool {
	return cmd == sessionListCmd || cmd == listCmd || cmd == backendCapabilitiesCmd || cmd == dashboardCmd
}
func prepareCommand(cmd *cobra.Command, args []string) error {
	selectedBackend = strings.ToLower(backendFlag)
	if selectedBackend != "" {
		if _, e := config.ParseBackend(selectedBackend); e != nil {
			return core.Failure("invalid_usage", "invalid explicit backend")
		}
	}
	if allBackends && (backendFlag != "" || !aggregateRead(cmd)) {
		return core.Failure("invalid_usage", "--all-backends requires list/capabilities/dashboard without --backend")
	}
	if jsonFlag && !isRead(cmd) && !isAgent(cmd) && !isExtension(cmd) && cmd != remoteEndpointCmd {
		return core.Failure("invalid_usage", "--json requires a supported session/backend/agent/extension command")
	}
	if isAgent(cmd) || isExtension(cmd) || cmd == remoteEndpointCmd || cmd == dashboardCmd {
		return nil
	}
	if cmd == profileCmd {
		return nil
	}
	if isRead(cmd) {
		if len(args) > 0 {
			if e := selectTargetBackend(args[0]); e != nil {
				return e
			}
		}
		return nil
	}
	if !needsBackend(cmd) {
		return nil
	}
	if len(args) > 0 && cmd != profileCmd {
		if e := selectTargetBackend(args[0]); e != nil {
			return e
		}
	}
	_, e := getManager(cmd.Context())
	return e
}
func selectTargetBackend(raw string) error {
	if !strings.Contains(raw, "/") {
		return nil
	}
	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[1] == "" {
		return core.Failure("invalid_target", "expected backend/name target")
	}
	if _, e := config.ParseBackend(parts[0]); e != nil {
		return core.Failure("invalid_target", "unknown target backend")
	}
	if backendFlag != "" && strings.ToLower(backendFlag) != parts[0] {
		return core.Failure("invalid_target", "target backend conflicts with --backend")
	}
	if allBackends {
		return core.Failure("invalid_usage", "qualified target conflicts with --all-backends")
	}
	selectedBackend = parts[0]
	return nil
}
func sessionTarget(raw string) (string, error) {
	canonical := false
	if strings.Contains(raw, "/") {
		parts := strings.Split(raw, "/")
		if len(parts) != 2 {
			return "", core.Failure("invalid_target", "expected backend/name target")
		}
		current := selectedBackend
		if manager != nil {
			current = manager.Backend.Name()
		}
		if parts[0] != current {
			return "", core.Failure("invalid_target", "target backend conflicts with selected backend")
		}
		raw = parts[1]
		canonical = true
	}
	current := selectedBackend
	if manager != nil {
		current = manager.Backend.Name()
	}
	if current == "tmux" && strings.HasPrefix(raw, "$") {
		if !regexp.MustCompile(`^\$[0-9]+$`).MatchString(raw) {
			return "", core.Failure("invalid_target", "invalid tmux session ID")
		}
		return raw, nil
	}
	if current == "screen" && strings.Contains(raw, ".") {
		if len(raw) > 512 || !regexp.MustCompile(`^[0-9]+\.[a-zA-Z0-9_.-]+$`).MatchString(raw) {
			return "", core.Failure("invalid_target", "invalid Screen session ID")
		}
		return raw, nil
	}
	if !canonical {
		raw = getSessionName(raw)
	}
	if e := validateName(raw); e != nil {
		return "", e
	}
	return raw, nil
}
func readBackends(cmd *cobra.Command) ([]backend.TerminalMultiplexer, error) {
	if allBackends {
		if _, e := config.LoadConfig(); e != nil {
			return nil, core.Failure("invalid_config", "could not load valid configuration")
		}
		out := []backend.TerminalMultiplexer{}
		for _, name := range backend.BackendNames() {
			b, e := backend.NewBackend(name, cmd.Context())
			if e != nil {
				return nil, e
			}
			out = append(out, b)
		}
		return out, nil
	}
	mgr, e := getManager(cmd.Context())
	if e != nil {
		return nil, e
	}
	return []backend.TerminalMultiplexer{mgr.Backend}, nil
}
func hostID(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, core.RequestTimeout)
	defer cancel()
	id, e := runtimepath.HostID(ctx)
	if e != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var known *core.OperationError
		if errors.As(e, &known) {
			return "", known
		}
		result := core.Failure("invalid_config", "private host identity could not be loaded or initialized")
		result.Cause = e
		return "", result
	}
	return id, nil
}
func queryResult(data any, failures []*core.OperationError, successes int) error {
	core.SortErrors(failures)
	partial := successes > 0 && len(failures) > 0
	if successes == 0 && len(failures) > 0 {
		data = nil
	}
	if jsonFlag {
		response = &core.Envelope{SchemaVersion: 1, Data: data, Errors: failures, Partial: partial}
	}
	if len(failures) > 0 {
		return &queryFailure{failures, partial}
	}
	return nil
}
func listSessions(cmd *cobra.Command, args []string) error {
	bs, e := readBackends(cmd)
	if e != nil {
		return e
	}
	id := ""
	if jsonFlag {
		id, e = hostID(cmd.Context())
		if e != nil {
			return e
		}
	}
	sessions := []core.Session{}
	failures := []*core.OperationError{}
	successes := 0
	for _, b := range bs {
		if !b.IsAvailable() {
			failures = append(failures, core.Scope(core.Failure("backend_unavailable", "backend executable/runtime unavailable"), b.Name()))
			continue
		}
		found, err := b.Sessions(cmd.Context())
		if err != nil {
			failures = append(failures, core.Scope(err, b.Name()))
			continue
		}
		successes++
		for _, s := range found {
			s.Locator.HostID = id
			sessions = append(sessions, s)
		}
	}
	if !allBackends && len(bs) == 1 && bs[0].Name() == "native" {
		if legacy := backend.LegacyNativeSessions(); len(legacy) > 0 {
			fmt.Fprintln(os.Stderr, "Warning: incompatible legacy native sessions (left untouched):", strings.Join(legacy, ", "))
		}
	}
	// Adapters sort their own observations; aggregate backend order is stable.
	if !jsonFlag {
		for _, s := range sessions {
			label := s.Locator.Name
			if allBackends {
				label = s.Locator.Backend + "/" + label
			}
			if s.Locator.Backend == "native" && s.AttachedClients != nil {
				fmt.Printf("%s [Attached: %d]\n", core.Plain([]byte(label)), *s.AttachedClients)
			} else if s.Locator.Backend == "tmux" && s.AttachedClients != nil {
				fmt.Printf("%s: [Attached: %d]\n", core.Plain([]byte(label)), *s.AttachedClients)
			} else {
				fmt.Println(core.Plain([]byte(label)))
			}
		}
	}
	return queryResult(struct {
		HostID     string         `json:"host_id"`
		ObservedAt time.Time      `json:"observed_at"`
		Sessions   []core.Session `json:"sessions"`
	}{id, time.Now().UTC(), sessions}, failures, successes)
}

var sessionInspectCmd = &cobra.Command{Use: "inspect TARGET", Short: "Inspect an exact session", Args: cobra.ExactArgs(1), ValidArgsFunction: getSingleSessionCompletion, RunE: func(cmd *cobra.Command, args []string) error {
	bs, e := readBackends(cmd)
	if e != nil {
		return e
	}
	name, e := parseSessionArgs(args)
	if e != nil {
		return e
	}
	id, e := hostID(cmd.Context())
	if e != nil {
		return e
	}
	s, e := bs[0].Lookup(cmd.Context(), name)
	if e != nil {
		return core.Scope(e, bs[0].Name())
	}
	s.Locator.HostID = id
	if !jsonFlag {
		fmt.Printf("%s/%s\nBackend ID: %s\nProcess: %s\nClients: %s\n", s.Locator.Backend, core.Plain([]byte(s.Locator.Name)), optional(s.BackendID), s.ProcessState, s.ClientState)
	}
	return queryResult(struct {
		HostID     string       `json:"host_id"`
		ObservedAt time.Time    `json:"observed_at"`
		Session    core.Session `json:"session"`
	}{id, time.Now().UTC(), s}, []*core.OperationError{}, 1)
}}

func optional(s *string) string {
	if s == nil {
		return "unknown"
	}
	return core.Plain([]byte(*s))
}

var backendCmd = &cobra.Command{Use: "backend", Short: "Inspect backend capabilities"}
var backendCapabilitiesCmd = &cobra.Command{Use: "capabilities", Short: "Report supported, degraded and unsupported operations", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
	bs, e := readBackends(cmd)
	if e != nil {
		return e
	}
	id, e := hostID(cmd.Context())
	if e != nil {
		return e
	}
	entries := []core.BackendCapabilities{}
	failures := []*core.OperationError{}
	successes := 0
	for _, b := range bs {
		c, e := b.BackendCapabilities(cmd.Context())
		entries = append(entries, c)
		if e != nil {
			failures = append(failures, core.Scope(e, b.Name()))
		} else {
			successes++
		}
	}
	if !jsonFlag {
		for _, c := range entries {
			fmt.Printf("%s (available: %t, version: %s)\n", c.Backend, c.Available, optional(c.Version))
			for _, key := range []string{"session_lifecycle", "attach", "read_only_attach", "output_snapshot", "output_history", "input_injection", "exact_pane_targeting", "window_layout", "working_directory", "managed_launch"} {
				cap := c.Capabilities[key]
				fmt.Printf("  %s: %s", key, cap.Level)
				if cap.Reason != nil {
					fmt.Printf(" (%s)", *cap.Reason)
				}
				fmt.Println()
			}
		}
	}
	return queryResult(struct {
		HostID     string                     `json:"host_id"`
		ObservedAt time.Time                  `json:"observed_at"`
		Backends   []core.BackendCapabilities `json:"backends"`
	}{id, time.Now().UTC(), entries}, failures, successes)
}}
var sessionOutputCmd = &cobra.Command{Use: "output TARGET", Short: "Capture bounded terminal output (safe plain text by default)", Args: cobra.ExactArgs(1), ValidArgsFunction: getSingleSessionCompletion, RunE: captureOutput}

func captureOutput(cmd *cobra.Command, args []string) error {
	o := core.CaptureOptions{Lines: outputLines, Bytes: outputBytes, Raw: outputRaw, History: outputHistory}
	if e := o.Validate(); e != nil {
		return e
	}
	if outputPlain && outputRaw {
		return core.Failure("invalid_usage", "--plain and --raw are mutually exclusive")
	}
	bs, e := readBackends(cmd)
	if e != nil {
		return e
	}
	name, e := parseSessionArgs(args)
	if e != nil {
		return e
	}
	id := ""
	if jsonFlag {
		id, e = hostID(cmd.Context())
		if e != nil {
			return e
		}
	}
	s, e := bs[0].Capture(cmd.Context(), name, o)
	if e != nil {
		return core.Scope(e, bs[0].Name())
	}
	s.Locator.HostID = id
	if !jsonFlag {
		data := []byte(s.Content)
		if outputRaw {
			data, e = base64.StdEncoding.DecodeString(s.Content)
			if e != nil {
				return e
			}
		}
		_, e = os.Stdout.Write(data)
		return e
	}
	return queryResult(s, []*core.OperationError{}, 1)
}
func preview(ctx context.Context, b backend.TerminalMultiplexer, name string, height int) string {
	ctx, cancel := context.WithTimeout(ctx, core.RequestTimeout)
	defer cancel()
	snap, e := b.Capture(ctx, name, core.CaptureOptions{Lines: max(1, min(height, 100)), Bytes: 32768})
	if e != nil {
		return "Preview unavailable: " + core.Normalize(e).Message
	}
	return snap.Content
}
func classify(err error) *core.OperationError {
	var native *p.Error
	if errors.As(err, &native) {
		code := native.Code
		if code == "limit_exceeded" {
			code = "invalid_usage"
		}
		return core.Failure(code, native.Message)
	}
	if strings.HasPrefix(err.Error(), "unknown command ") {
		return core.Failure("invalid_usage", "unknown command")
	}
	return core.Normalize(err)
}
func finish(err error, jsonMode bool) int {
	failures := []*core.OperationError{}
	partial := false
	if err != nil {
		var query *queryFailure
		if errors.As(err, &query) {
			failures, partial = query.failures, query.partial
		} else {
			failures = append(failures, classify(err))
		}
	}
	code := core.ExitCode(failures, partial)
	if jsonMode {
		if response == nil {
			response = &core.Envelope{SchemaVersion: 1, Errors: failures, Partial: partial}
		}
		if e := json.NewEncoder(os.Stdout).Encode(response); e != nil {
			fmt.Fprintln(os.Stderr, "Error: could not write JSON response")
			return 1
		}
	} else {
		for _, e := range failures {
			fmt.Fprintln(os.Stderr, "Error:", e.Message)
		}
	}
	return code
}
func jsonRequested(args []string) bool {
	if cmd, _, e := rootCmd.Find(args); e == nil && !isRead(cmd) && !isAgent(cmd) && !isExtension(cmd) && cmd != remoteEndpointCmd {
		return false
	}
	result := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--json" || arg == "--json=true" {
			result = true
		}
		if arg == "--json=false" {
			result = false
		}
	}
	return result
}
func wrapArguments(cmd *cobra.Command) {
	if cmd.Run == nil && cmd.RunE == nil && cmd.HasSubCommands() {
		cmd.RunE = func(c *cobra.Command, args []string) error {
			if jsonFlag {
				return core.Failure("invalid_usage", "select a supported read subcommand")
			}
			return c.Help()
		}
	}
	if cmd.Args != nil {
		original := cmd.Args
		cmd.Args = func(c *cobra.Command, args []string) error {
			if e := original(c, args); e != nil {
				return core.Failure("invalid_usage", "invalid arguments for "+c.Name())
			}
			return nil
		}
	}
	for _, child := range cmd.Commands() {
		wrapArguments(child)
	}
}
func init() {
	paneExecCmd.Flags().SetInterspersed(false)
	execCmd.Flags().SetInterspersed(false)
	rootCmd.PersistentFlags().StringVar(&backendFlag, "backend", "", "Select one backend strictly (no fallback)")
	rootCmd.PersistentFlags().BoolVar(&allBackends, "all-backends", false, "Aggregate list/capability reads or dashboard scope across all backends")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Emit a versioned JSON envelope for supported session/backend/agent/extension commands")
	rootCmd.SetFlagErrorFunc(func(*cobra.Command, error) error { return core.Failure("invalid_usage", "invalid command flags") })
	sessionCmd.AddCommand(sessionInspectCmd, sessionOutputCmd)
	rootCmd.AddCommand(backendCmd)
	_ = rootCmd.RegisterFlagCompletionFunc("backend", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return backend.BackendNames(), cobra.ShellCompDirectiveNoFileComp
	})
	backendCmd.AddCommand(backendCapabilitiesCmd)
	for _, cmd := range []*cobra.Command{sessionOutputCmd, sessionDumpCmd, dumpCmd} {
		cmd.Flags().IntVar(&outputLines, "lines", 100, "Maximum lines (1..10000)")
		cmd.Flags().IntVar(&outputBytes, "bytes", core.MaxBytes, "Maximum decoded bytes (1..1048576)")
		cmd.Flags().BoolVar(&outputPlain, "plain", false, "Safe plain UTF-8 output (default)")
		cmd.Flags().BoolVar(&outputRaw, "raw", false, "Raw terminal sequences (explicit opt-in)")
		cmd.Flags().BoolVar(&outputHistory, "history", false, "Request bounded history where supported")
	}
}
