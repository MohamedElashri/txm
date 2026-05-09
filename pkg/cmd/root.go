package cmd

import (
	"fmt"
	"os"
	"regexp"

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

var rootCmd = &cobra.Command{
	Use:   "txm",
	Short: "A Terminal Session Manager",
	Long:  `txm is a powerful command-line utility designed to manage terminal multiplexer sessions efficiently. It supports tmux, zellij, and GNU Screen.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Just validate that we can get a manager before running any subcommands (except help/completion)
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}
		_, err := getManager()
		return err
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
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(detachCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(renameSessionCmd)
	rootCmd.AddCommand(nukeCmd)

	// Window Management
	rootCmd.AddCommand(newWindowCmd)
	rootCmd.AddCommand(listWindowsCmd)
	rootCmd.AddCommand(killWindowCmd)
	rootCmd.AddCommand(nextWindowCmd)
	rootCmd.AddCommand(prevWindowCmd)
	rootCmd.AddCommand(renameWindowCmd)
	rootCmd.AddCommand(moveWindowCmd)
	rootCmd.AddCommand(swapWindowCmd)
	rootCmd.AddCommand(splitWindowCmd)

	// Pane Management
	rootCmd.AddCommand(listPanesCmd)
	rootCmd.AddCommand(killPaneCmd)
	rootCmd.AddCommand(resizePaneCmd)
	rootCmd.AddCommand(sendKeysCmd)

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
