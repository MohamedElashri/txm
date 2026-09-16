package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"

	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/MohamedElashri/txm/pkg/logger"
)

var (
	verbosity   int
	manager     *backend.Manager
	logInstance *logger.Logger
)

func isValidName(name string) bool {
	// Only allow alphanumeric characters, dashes, and underscores
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return match
}

func validateName(name string) error {
	if !isValidName(name) {
		return fmt.Errorf("invalid name '%s': only alphanumeric characters, dashes, and underscores are allowed", name)
	}
	return nil
}

func getSessionName(name string) string {
	prefix := os.Getenv("TXM_SESSION_PREFIX")
	if prefix != "" && !strings.HasPrefix(name, prefix) {
		return prefix + name
	}
	return name
}

func getManager() (*backend.Manager, error) {
	if manager != nil {
		return manager, nil
	}

	cfg, err := config.LoadConfig()
	if err != nil && verbosity > 0 {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		cfg = config.NewDefaultConfig()
	}

	logInstance = logger.NewLogger(verbosity)
	manager = backend.NewManager(cfg, logInstance)

	if err := manager.CheckAvailability(); err != nil {
		return nil, err
	}

	return manager, nil
}

func getSessionCompletions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	mgr, err := getManager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	sessions, err := mgr.Backend.GetSessions()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return sessions, cobra.ShellCompDirectiveNoFileComp
}

func getSingleSessionCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getSessionCompletions(cmd, args, toComplete)
}

var rootCmd = &cobra.Command{
	Use:   "txm",
	Short: "A Terminal Session Manager",
	Long:  `txm is a powerful command-line utility designed to manage terminal multiplexer sessions efficiently. It supports tmux, zellij, and GNU Screen.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Just validate that we can get a manager before running any subcommands (except help/completion)
		if cmd.Name() == "help" || cmd.Name() == "completion" || cmd.Name() == "txm" {
			return nil
		}
		_, err := getManager()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getManager()
		if err != nil {
			return err
		}

		sessions, err := mgr.Backend.GetSessions()
		if err != nil || len(sessions) == 0 {
			return cmd.Help()
		}

		idx, err := fuzzyfinder.Find(
			sessions,
			func(i int) string {
				return sessions[i]
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				out, _ := mgr.Backend.DumpSession(sessions[i])
				return out
			}),
		)
		if err != nil {
			if err == fuzzyfinder.ErrAbort {
				return nil
			}
			return err
		}

		return mgr.Backend.AttachSession(sessions[idx])
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Legacy and Alias Commands

var createCmd = &cobra.Command{
	Use:               "create [session_name] [command...]",
	Short:             "Create a new session (alias for 'txm session create')",
	Args:              sessionCreateCmd.Args,
	ValidArgsFunction: sessionCreateCmd.ValidArgsFunction,
	RunE:              sessionCreateCmd.RunE,
}

var listCmd = &cobra.Command{
	Use:               "list",
	Short:             "List all active sessions (alias for 'txm session list')",
	Args:              sessionListCmd.Args,
	RunE:              sessionListCmd.RunE,
}

var attachCmd = &cobra.Command{
	Use:               "attach [session_name] [command...]",
	Short:             "Attach to an existing session (alias for 'txm session attach')",
	Args:              sessionAttachCmd.Args,
	ValidArgsFunction: sessionAttachCmd.ValidArgsFunction,
	RunE:              sessionAttachCmd.RunE,
}

var detachCmd = &cobra.Command{
	Use:   "detach",
	Short: "Detach from current session (alias for 'txm session detach')",
	Args:  sessionDetachCmd.Args,
	RunE:  sessionDetachCmd.RunE,
}

var deleteCmd = &cobra.Command{
	Use:               "delete [session_name]",
	Short:             "Delete a session (alias for 'txm session delete')",
	Args:              sessionDeleteCmd.Args,
	ValidArgsFunction: sessionDeleteCmd.ValidArgsFunction,
	RunE:              sessionDeleteCmd.RunE,
}

var renameSessionCmd = &cobra.Command{
	Use:               "rename-session [old_name] [new_name]",
	Short:             "Rename an existing session (alias for 'txm session rename')",
	Args:              sessionRenameCmd.Args,
	ValidArgsFunction: sessionRenameCmd.ValidArgsFunction,
	RunE:              sessionRenameCmd.RunE,
}

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Remove all sessions (alias for 'txm session nuke')",
	Args:  sessionNukeCmd.Args,
	RunE:  sessionNukeCmd.RunE,
}

var execCmd = &cobra.Command{
	Use:               "exec [session_name] [window_name] [pane_id] [command]",
	Short:             "Execute a command in a pane (alias for 'txm pane exec')",
	Args:              paneExecCmd.Args,
	ValidArgsFunction: paneExecCmd.ValidArgsFunction,
	RunE:              paneExecCmd.RunE,
}

var dumpCmd = &cobra.Command{
	Use:               "dump [session_name]",
	Hidden:            true,
	Args:              sessionDumpCmd.Args,
	ValidArgsFunction: sessionDumpCmd.ValidArgsFunction,
	RunE:              sessionDumpCmd.RunE,
}

var generateSshConfigCmd = &cobra.Command{
	Use:   "generate-ssh-config",
	Short: "Generate SSH config snippet for seamless remote workflows",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		config := `
# Add this to your ~/.ssh/config for seamless session persistence

Host d.*
    HostName 192.168.1.xxx

    # Automatically attach to a txm session on the remote server
    # named after the remote host we're connecting to (%h).
    RemoteCommand txm attach %h
    RequestTTY yes

    # Multiplex multiple PTY sessions to a single server over one connection
    ControlMaster auto
    ControlPath ~/.ssh/cm-%r@%h:%p
    ControlPersist 10m
`
		fmt.Println(strings.TrimSpace(config))
	},
}

func init() {
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "verbosity level (can be specified multiple times, e.g., -vv)")

	// Group Commands
	rootCmd.AddCommand(sessionCmd)
	rootCmd.AddCommand(windowCmd)
	rootCmd.AddCommand(paneCmd)

	// Add flags to session subcommands
	sessionCreateCmd.Flags().SetInterspersed(false)
	sessionCreateCmd.Flags().StringVarP(&createLogFile, "log", "l", "", "Log session output to a file")
	sessionAttachCmd.Flags().SetInterspersed(false)
	sessionAttachCmd.Flags().BoolVarP(&attachReadOnly, "read-only", "r", false, "Attach in read-only mode")

	// Root Level Aliases (Non-deprecated)
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().SetInterspersed(false)
	createCmd.Flags().StringVarP(&createLogFile, "log", "l", "", "Log session output to a file")

	rootCmd.AddCommand(listCmd)

	rootCmd.AddCommand(attachCmd)
	attachCmd.Flags().SetInterspersed(false)
	attachCmd.Flags().BoolVarP(&attachReadOnly, "read-only", "r", false, "Attach in read-only mode")

	// Root Level Aliases
	rootCmd.AddCommand(detachCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(renameSessionCmd)
	rootCmd.AddCommand(nukeCmd)
	rootCmd.AddCommand(execCmd)
	
	// Server command (from server.go)
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().SetInterspersed(false)

	// Hidden / Misc
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(generateSshConfigCmd)
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configShowCmd)

	rootCmd.AddCommand(installCmd)
	installCmd.Flags().Bool("system", false, "Install system-wide (requires root)")

	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
}
