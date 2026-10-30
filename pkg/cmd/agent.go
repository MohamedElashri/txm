package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{Use: "agent", Short: "Opt-in supervised agents and local state"}
var agentCwd string
var agentEnv []string
var agentProvider string

func isAgent(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Parent() == agentCmd {
			return true
		}
	}
	return false
}
func agentResult(reply agent.Reply, e error, single bool) error {
	if reply.Agents == nil {
		reply.Agents = []agent.View{}
	}
	var data any = struct {
		Agents []agent.View `json:"agents"`
	}{reply.Agents}
	if single && len(reply.Agents) > 0 {
		data = struct {
			Agent agent.View `json:"agent"`
		}{reply.Agents[0]}
	}
	if !jsonFlag {
		for _, v := range reply.Agents {
			fmt.Printf("%s %s/%s %s (attention: %s, source: %s)\n", v.ID, v.Locator.Backend, core.Plain([]byte(v.Locator.Name)), v.Lifecycle, v.Attention, v.Source)
		}
	}
	if e != nil {
		return queryResult(data, []*core.OperationError{core.Normalize(e)}, len(reply.Agents))
	}
	return queryResult(data, []*core.OperationError{}, 1)
}

var agentStartCmd = &cobra.Command{Use: "start NAME -- EXECUTABLE [ARGS...]", Short: "Start a supervised command in a new session", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
	if cmd.ArgsLenAtDash() != 1 {
		return core.Failure("invalid_usage", "agent start requires NAME -- EXECUTABLE [ARGS...]")
	}
	if !supervisor.Supported() {
		return core.Failure("unsupported", "managed agents require Unix and CGO")
	}
	name := getSessionName(args[0])
	mgr, e := getManager(cmd.Context())
	if e != nil {
		return e
	}
	if e = backend.PreflightManaged(cmd.Context(), mgr.Backend); e != nil {
		return e
	}
	executable, e := exec.LookPath(args[1])
	if e != nil {
		return core.Failure("invalid_usage", "managed executable is unavailable")
	}
	executable, e = filepath.Abs(executable)
	if e != nil {
		return e
	}
	argv := append([]string{executable}, args[2:]...)
	names := []string{"HOME", "PATH", "TERM"}
	seen := map[string]bool{"HOME": true, "PATH": true, "TERM": true}
	for _, name := range agentEnv {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	m, e := agent.CompileStart(name, mgr.Backend.Name(), agentCwd, argv, names)
	if e != nil {
		return e
	}
	provider := agent.Provider{}
	if agentProvider != "" {
		if agentProvider != "generic" && agentProvider != "claude" {
			return core.Failure("unsupported", "automatic provider configuration unavailable; use reviewed hooks configuration")
		}
		provider = agent.InspectProvider(cmd.Context(), agentProvider)
		if !provider.Supported {
			return core.Failure("unsupported_version", provider.Reason)
		}
		if agentProvider == "claude" {
			installed, err := exec.LookPath("claude")
			if err != nil {
				return err
			}
			installed, err = filepath.EvalSymlinks(installed)
			if err != nil {
				return err
			}
			actual, err := filepath.EvalSymlinks(executable)
			if err != nil {
				return err
			}
			if actual != installed {
				return core.Failure("invalid_usage", "--provider claude requires the verified claude executable")
			}
		}
	}
	reply, e := agent.Call(cmd.Context(), agent.Request{Provider: agentProvider, ProviderVersion: provider.Version, Op: "start", Name: name, Backend: m.Backend, Argv: m.Argv, Cwd: m.Cwd, EnvNames: m.EnvNames}, true)
	return agentResult(reply, e, true)
}}
var agentRegisterCmd = &cobra.Command{Use: "register TARGET", Short: "Register an existing session without supervision", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	if e := selectTargetBackend(args[0]); e != nil {
		return e
	}
	mgr, e := getManager(cmd.Context())
	if e != nil {
		return e
	}
	if e = backend.PreflightManaged(cmd.Context(), mgr.Backend); e != nil {
		return e
	}
	name, e := sessionTarget(args[0])
	if e != nil {
		return e
	}
	s, e := mgr.Backend.Lookup(cmd.Context(), name)
	if e != nil {
		return e
	}
	reply, e := agent.Call(cmd.Context(), agent.Request{Op: "register", Name: s.Locator.Name, Backend: mgr.Backend.Name()}, true)
	return agentResult(reply, e, true)
}}
var agentListCmd = &cobra.Command{Use: "list", Short: "List registrations and reconcile supervised state", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	reply, e := agent.Call(cmd.Context(), agent.Request{Op: "list", Backend: selectedBackend}, true)
	return agentResult(reply, e, false)
}}
var agentStatusCmd = &cobra.Command{Use: "status ID", Short: "Inspect one exact agent registration ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	if !runtimepath.ValidID(args[0]) {
		return core.Failure("invalid_target", "agent status requires a full opaque ID")
	}
	reply, e := agent.Call(cmd.Context(), agent.Request{Op: "status", ID: args[0], Backend: selectedBackend}, true)
	return agentResult(reply, e, true)
}}
var agentStopCoordinatorCmd = &cobra.Command{Use: "stop-coordinator", Short: "Stop local agent state service; leave sessions running", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	if backendFlag != "" {
		return core.Failure("invalid_usage", "coordinator is independent of backend selection")
	}
	reply, e := agent.Call(cmd.Context(), agent.Request{Op: "stop"}, false)
	return agentResult(reply, e, false)
}}
var internalCmd = &cobra.Command{Use: "internal", Hidden: true}
var supervisedRunCmd = &cobra.Command{Use: "supervised-run INSTANCE", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGHUP, syscall.SIGTERM, os.Interrupt)
	defer stop()
	e := supervisor.Run(ctx, args[0])
	if e == nil {
		agent.Notify()
	}
	return e
}}
var coordinatorCmd = &cobra.Command{Use: "coordinator", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	epoch, _ := cmd.Flags().GetString("epoch")
	fd, _ := cmd.Flags().GetInt("ready-fd")
	if fd != -1 && fd != 3 {
		return core.Failure("invalid_usage", "invalid coordinator readiness descriptor")
	}
	if epoch == "" {
		var e error
		epoch, e = runtimepath.NewID()
		if e != nil {
			return e
		}
	}
	var ready *os.File
	if fd == 3 {
		ready = os.NewFile(3, "coordinator-ready")
		defer func() { _ = ready.Close() }()
	}
	notify := func(e error) {
		if ready != nil {
			message := ""
			if e != nil {
				message = "coordinator startup failed"
			}
			_ = json.NewEncoder(ready).Encode(struct {
				Epoch string `json:"epoch"`
				Error string `json:"error"`
			}{epoch, message})
		}
	}
	return agent.Serve(cmd.Context(), epoch, notify)
}}

func init() {
	agentStartCmd.Flags().StringVar(&agentProvider, "provider", "", "Opt in to generic or verified Claude hooks")
	rootCmd.AddCommand(agentCmd, internalCmd)
	agentCmd.AddCommand(agentStartCmd, agentRegisterCmd, agentListCmd, agentStatusCmd, agentStopCoordinatorCmd)
	agentStartCmd.Flags().StringVar(&agentCwd, "cwd", "", "Working directory (default invocation cwd)")
	agentStartCmd.Flags().StringArrayVar(&agentEnv, "env", nil, "Allow a variable name from the backend wrapper environment (repeatable)")
	internalCmd.AddCommand(supervisedRunCmd, coordinatorCmd)
	coordinatorCmd.Flags().String("epoch", "", "Internal coordinator epoch")
	coordinatorCmd.Flags().Int("ready-fd", -1, "Internal readiness descriptor")
}
