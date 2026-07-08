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
	verbose     bool
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
	if err != nil && verbose {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		cfg = config.NewDefaultConfig()
	}

	logInstance = logger.NewLogger(verbose)
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

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Session Management
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().SetInterspersed(false)
	createCmd.Flags().StringVarP(&createLogFile, "log", "l", "", "Log session output to a file")
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(attachCmd)
	attachCmd.Flags().SetInterspersed(false)
	attachCmd.Flags().BoolVarP(&attachReadOnly, "read-only", "r", false, "Attach in read-only mode")
	rootCmd.AddCommand(detachCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(renameSessionCmd)
	rootCmd.AddCommand(nukeCmd)
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(generateSshConfigCmd)

	// Window Management
	var windowCmd = &cobra.Command{
		Use:   "window",
		Short: "Manage windows within a session",
	}
	rootCmd.AddCommand(windowCmd)
	windowCmd.AddCommand(newWindowCmd)
	windowCmd.AddCommand(listWindowsCmd)
	windowCmd.AddCommand(killWindowCmd)
	windowCmd.AddCommand(nextWindowCmd)
	windowCmd.AddCommand(prevWindowCmd)
	windowCmd.AddCommand(renameWindowCmd)
	windowCmd.AddCommand(splitWindowCmd)

	// Pane Management
	var paneCmd = &cobra.Command{
		Use:   "pane",
		Short: "Manage panes within a window",
	}
	rootCmd.AddCommand(paneCmd)
	paneCmd.AddCommand(listPanesCmd)
	paneCmd.AddCommand(killPaneCmd)

	// Misc
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
