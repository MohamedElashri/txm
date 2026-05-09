package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MohamedElashri/txm/docs"
	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		cfg, err := config.LoadConfig()
		if err != nil {
			cfg = config.NewDefaultConfig()
		}

		if key == "backend" || key == "default_backend" {
			backendType, err := config.ParseBackend(value)
			if err != nil {
				return fmt.Errorf("invalid backend: %s. Must be tmux, zellij, or screen", value)
			}
			cfg.DefaultBackend = backendType
		} else {
			return fmt.Errorf("unknown configuration key: %s", key)
		}

		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save configuration: %v", err)
		}

		fmt.Printf("Successfully set %s to %s\n", key, value)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		cfg, err := config.LoadConfig()
		if err != nil {
			cfg = config.NewDefaultConfig()
		}

		if key == "backend" || key == "default_backend" {
			fmt.Println(cfg.DefaultBackend)
		} else {
			return fmt.Errorf("unknown configuration key: %s", key)
		}
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show all configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			cfg = config.NewDefaultConfig()
		}

		fmt.Println("txm Configuration:")
		fmt.Printf("  Default Backend: %s\n", cfg.DefaultBackend)
		fmt.Printf("  Backend Order:   %v\n", cfg.BackendOrder)
		return nil
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install txm binary and man page",
	RunE: func(cmd *cobra.Command, args []string) error {
		system, _ := cmd.Flags().GetBool("system")

		var binDir, manDir string
		if system {
			if os.Getuid() != 0 {
				return fmt.Errorf("system installation requires root privileges. Please run with sudo")
			}
			binDir = "/usr/local/bin"
			manDir = "/usr/local/share/man/man1"
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %v", err)
			}
			binDir = filepath.Join(homeDir, ".local/bin")
			manDir = filepath.Join(homeDir, ".local/share/man/man1")
		}

		// Create directories
		if err := os.MkdirAll(binDir, 0755); err != nil {
			return fmt.Errorf("failed to create bin directory: %v", err)
		}
		if err := os.MkdirAll(manDir, 0755); err != nil {
			return fmt.Errorf("failed to create man directory: %v", err)
		}

		// Install Binary
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to find executable: %v", err)
		}
		
		destBin := filepath.Join(binDir, "txm")
		if err := copyFile(execPath, destBin); err != nil {
			return fmt.Errorf("failed to install binary: %v", err)
		}
		os.Chmod(destBin, 0755)

		// Install Man Page
		destMan := filepath.Join(manDir, "txm.1")
		if err := os.WriteFile(destMan, docs.ManPageContent, 0644); err != nil {
			return fmt.Errorf("failed to install man page: %v", err)
		}

		fmt.Printf("Successfully installed txm to %s\n", destBin)
		fmt.Printf("Successfully installed man page to %s\n", destMan)

		// Install Completions
		installCompletions(system)

		return nil
	},
}

func installCompletions(system bool) {
	var bashPath, zshPath, fishPath string
	if system {
		bashPath = "/usr/share/bash-completion/completions/txm"
		zshPath = "/usr/share/zsh/site-functions/_txm"
		fishPath = "/usr/share/fish/vendor_completions.d/txm.fish"
	} else {
		homeDir, _ := os.UserHomeDir()
		bashPath = filepath.Join(homeDir, ".local/share/bash-completion/completions/txm")
		zshPath = filepath.Join(homeDir, ".zfunc/_txm")
		fishPath = filepath.Join(homeDir, ".config/fish/completions/txm.fish")
	}

	// Bash
	os.MkdirAll(filepath.Dir(bashPath), 0755)
	if f, err := os.Create(bashPath); err == nil {
		rootCmd.GenBashCompletion(f)
		f.Close()
		fmt.Printf("Installed bash completion to %s\n", bashPath)
	}

	// Zsh
	os.MkdirAll(filepath.Dir(zshPath), 0755)
	if f, err := os.Create(zshPath); err == nil {
		rootCmd.GenZshCompletion(f)
		f.Close()
		fmt.Printf("Installed zsh completion to %s\n", zshPath)
		if !system {
			fmt.Printf("  (Note: Make sure `fpath+=~/.zfunc` is in your ~/.zshrc before compinit)\n")
		}
	}

	// Fish
	os.MkdirAll(filepath.Dir(fishPath), 0755)
	if f, err := os.Create(fishPath); err == nil {
		rootCmd.GenFishCompletion(f, true)
		f.Close()
		fmt.Printf("Installed fish completion to %s\n", fishPath)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update txm to the latest version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Updating txm...")
		fmt.Println("For installations managed by GoReleaser, please update via your package manager or download the latest release from GitHub.")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall txm and its man page",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %v", err)
		}

		isSystem := strings.HasPrefix(execPath, "/usr/local/bin")
		if isSystem && os.Getuid() != 0 {
			return fmt.Errorf("system uninstallation requires root privileges. Please run with sudo")
		}

		var manPath string
		if isSystem {
			manPath = "/usr/local/share/man/man1/txm.1"
		} else {
			homeDir, _ := os.UserHomeDir()
			manPath = filepath.Join(homeDir, ".local/share/man/man1/txm.1")
		}

		// Remove binary
		if err := os.Remove(execPath); err != nil {
			fmt.Printf("Failed to remove binary at %s: %v\n", execPath, err)
		} else {
			fmt.Printf("Removed binary at %s\n", execPath)
		}

		// Remove man page
		if err := os.Remove(manPath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("Failed to remove man page at %s: %v\n", manPath, err)
			}
		} else {
			fmt.Printf("Removed man page at %s\n", manPath)
		}

		// Remove Completions
		var bashPath, zshPath, fishPath string
		if isSystem {
			bashPath = "/usr/share/bash-completion/completions/txm"
			zshPath = "/usr/share/zsh/site-functions/_txm"
			fishPath = "/usr/share/fish/vendor_completions.d/txm.fish"
		} else {
			homeDir, _ := os.UserHomeDir()
			bashPath = filepath.Join(homeDir, ".local/share/bash-completion/completions/txm")
			zshPath = filepath.Join(homeDir, ".zfunc/_txm")
			fishPath = filepath.Join(homeDir, ".config/fish/completions/txm.fish")
		}

		for _, p := range []string{bashPath, zshPath, fishPath} {
			if err := os.Remove(p); err == nil {
				fmt.Printf("Removed completion at %s\n", p)
			}
		}

		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version and check for updates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("txm version 1.0.0") // Ideally injected at build time via ldflags
		return nil
	},
}
