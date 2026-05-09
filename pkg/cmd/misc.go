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

		// Resolve symlinks to compare actual paths
		realExecPath, err := filepath.EvalSymlinks(execPath)
		if err != nil {
			realExecPath = execPath
		}

		destBin := filepath.Join(binDir, "txm")
		realDestBin, err := filepath.EvalSymlinks(destBin)
		if err != nil {
			realDestBin = destBin
		}

		if realExecPath != realDestBin {
			if err := copyFile(execPath, destBin); err != nil {
				return fmt.Errorf("failed to install binary: %v", err)
			}
			if err := os.Chmod(destBin, 0755); err != nil {
				return fmt.Errorf("failed to set binary permissions: %v", err)
			}
		} else {
			fmt.Printf("Binary already at %s, skipping copy\n", destBin)
		}

		// Install Man Page
		destMan := filepath.Join(manDir, "txm.1")
		if err := os.WriteFile(destMan, docs.ManPageContent, 0644); err != nil {
			return fmt.Errorf("failed to install man page: %v", err)
		}

		fmt.Printf("Successfully installed txm to %s\n", destBin)
		fmt.Printf("Successfully installed man page to %s\n", destMan)

		// Detect current shell to only install what's needed
		shellEnv := os.Getenv("SHELL")
		detectedShell := ""
		if strings.Contains(shellEnv, "zsh") {
			detectedShell = "zsh"
		} else if strings.Contains(shellEnv, "bash") {
			detectedShell = "bash"
		} else if strings.Contains(shellEnv, "fish") {
			detectedShell = "fish"
		}

		// Install Completions
		installCompletions(system, detectedShell)

		return nil
	},
}

func installCompletions(system bool, shell string) {
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
	if shell == "" || shell == "bash" {
		if err := os.MkdirAll(filepath.Dir(bashPath), 0755); err == nil {
			if f, err := os.Create(bashPath); err == nil {
				_ = rootCmd.GenBashCompletion(f)
				_ = f.Close()
				fmt.Printf("Installed bash completion to %s\n", bashPath)
				if !system {
					// Try to automatically source in .bashrc
					homeDir, _ := os.UserHomeDir()
					bashrcPath := filepath.Join(homeDir, ".bashrc")
					if _, err := os.Stat(bashrcPath); err == nil {
						content, _ := os.ReadFile(bashrcPath)
						if !strings.Contains(string(content), "txm completion bash") && !strings.Contains(string(content), bashPath) {
							f, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
							if err == nil {
								_, _ = fmt.Fprintf(f, "\n# Added by txm\n[[ -f %q ]] && . %q\n", bashPath, bashPath)
								_ = f.Close()
								fmt.Printf("Added source line for bash completion in %s\n", bashrcPath)
								if strings.Contains(os.Getenv("SHELL"), "bash") {
									fmt.Printf("Please run 'source %s' to activate it.\n", bashrcPath)
								}
							}
						}
					}
				}
			}
		}
	}

	// Zsh
	if shell == "" || shell == "zsh" {
		if err := os.MkdirAll(filepath.Dir(zshPath), 0755); err == nil {
			if f, err := os.Create(zshPath); err == nil {
				_ = rootCmd.GenZshCompletion(f)
				_ = f.Close()
				fmt.Printf("Installed zsh completion to %s\n", zshPath)
				if !system {
					// Try to automatically add to fpath in .zshrc
					homeDir, _ := os.UserHomeDir()
					zshrcPath := filepath.Join(homeDir, ".zshrc")
					if _, err := os.Stat(zshrcPath); err == nil {
						content, _ := os.ReadFile(zshrcPath)
						if !strings.Contains(string(content), "fpath+=~/.zfunc") && !strings.Contains(string(content), "fpath+=(~/.zfunc)") {
							f, err := os.OpenFile(zshrcPath, os.O_APPEND|os.O_WRONLY, 0644)
							if err == nil {
								_, _ = f.WriteString("\n# Added by txm\nfpath+=~/.zfunc\n")
								_ = f.Close()
								fmt.Printf("Added ~/.zfunc to fpath in %s\n", zshrcPath)
								if strings.Contains(os.Getenv("SHELL"), "zsh") {
									fmt.Printf("IMPORTANT: Ensure 'fpath+=~/.zfunc' is ABOVE 'compinit' in your %s\n", zshrcPath)
									fmt.Printf("Then run: rm -f ~/.zcompdump; compinit\n")
								}
							}
						}
					} else {
						fmt.Printf("  (Note: Make sure `fpath+=~/.zfunc` is in your ~/.zshrc before compinit)\n")
					}
				}
			}
		}
	}

	// Fish
	if shell == "" || shell == "fish" {
		if err := os.MkdirAll(filepath.Dir(fishPath), 0755); err == nil {
			if f, err := os.Create(fishPath); err == nil {
				_ = rootCmd.GenFishCompletion(f, true)
				_ = f.Close()
				fmt.Printf("Installed fish completion to %s\n", fishPath)
			}
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if closeErr := out.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
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

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `To load completions for your current session:
$ source <(txm completion bash)
$ source <(txm completion zsh)

To install them permanently, use the --install flag:
$ txm completion zsh --install`,
}

var completionInstall bool

func init() {
	completionCmd.PersistentFlags().BoolVar(&completionInstall, "install", false, "Install completion script to standard location")

	completionCmd.AddCommand(&cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		Run: func(cmd *cobra.Command, args []string) {
			if completionInstall {
				installCompletions(false, "bash")
				return
			}
			// Check if stdout is a terminal
			if stat, _ := os.Stdout.Stat(); (stat.Mode() & os.ModeCharDevice) != 0 {
				fmt.Fprintln(os.Stderr, "Error: You are running this in a terminal. To install completion, use:")
				fmt.Fprintln(os.Stderr, "  txm completion bash --install")
				fmt.Fprintln(os.Stderr, "\nOr to test in current session:")
				fmt.Fprintln(os.Stderr, "  source <(txm completion bash)")
				return
			}
			_ = rootCmd.GenBashCompletion(os.Stdout)
		},
	})

	completionCmd.AddCommand(&cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		Run: func(cmd *cobra.Command, args []string) {
			if completionInstall {
				installCompletions(false, "zsh")
				return
			}
			// Check if stdout is a terminal
			if stat, _ := os.Stdout.Stat(); (stat.Mode() & os.ModeCharDevice) != 0 {
				fmt.Fprintln(os.Stderr, "Error: You are running this in a terminal. To install completion, use:")
				fmt.Fprintln(os.Stderr, "  txm completion zsh --install")
				fmt.Fprintln(os.Stderr, "\nOr to test in current session:")
				fmt.Fprintln(os.Stderr, "  source <(txm completion zsh)")
				return
			}
			_ = rootCmd.GenZshCompletion(os.Stdout)
		},
	})

	completionCmd.AddCommand(&cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		Run: func(cmd *cobra.Command, args []string) {
			if completionInstall {
				installCompletions(false, "fish")
				return
			}
			// Check if stdout is a terminal
			if stat, _ := os.Stdout.Stat(); (stat.Mode() & os.ModeCharDevice) != 0 {
				fmt.Fprintln(os.Stderr, "Error: You are running this in a terminal. To install completion, use:")
				fmt.Fprintln(os.Stderr, "  txm completion fish --install")
				fmt.Fprintln(os.Stderr, "\nOr to test in current session:")
				fmt.Fprintln(os.Stderr, "  txm completion fish | source")
				return
			}
			_ = rootCmd.GenFishCompletion(os.Stdout, true)
		},
	})
}

// Version is set at build time via ldflags: -X github.com/MohamedElashri/txm/pkg/cmd.Version=<tag>
var Version = "1.0.5"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version and check for updates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("txm version %s\n", Version)
		return nil
	},
}
