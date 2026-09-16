package cmd

import (
	"fmt"

	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile [path_to_toml]",
	Short: "Load and start a session from a declarative TOML profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profilePath := args[0]
		prof, err := config.LoadProfile(profilePath)
		if err != nil {
			return fmt.Errorf("failed to load profile: %v", err)
		}

		if prof.Name == "" {
			return fmt.Errorf("profile must have a 'name'")
		}

		if manager.Backend.SessionExists(prof.Name) {
			logInstance.Warning(fmt.Sprintf("Session '%s' already exists, attaching...", prof.Name))
			return manager.Backend.AttachSession(prof.Name)
		}

		logInstance.Info(fmt.Sprintf("Creating session '%s' from profile...", prof.Name))
		if err := manager.Backend.CreateSession(prof.Name); err != nil {
			return fmt.Errorf("failed to create session: %v", err)
		}

		for i, win := range prof.Windows {
			if i == 0 {
				if win.Name != "" {
					_ = manager.Backend.RenameWindow(prof.Name, "0", win.Name)
					_ = manager.Backend.RenameWindow(prof.Name, "1", win.Name)
				}
			} else {
				if err := manager.Backend.NewWindow(prof.Name, win.Name); err != nil {
					return fmt.Errorf("failed to create window '%s': %v", win.Name, err)
				}
			}

			for j, pane := range win.Panes {
				if j > 0 {
					splitDir := pane.Split
					if splitDir == "" {
						splitDir = "v"
					}
					if err := manager.Backend.SplitWindow(prof.Name, win.Name, splitDir); err != nil {
						logInstance.Warning(fmt.Sprintf("Failed to split window: %v", err))
					}
				}

				if pane.Command != "" {
					// We pass an empty pane string assuming it targets the active pane (the one just created).
					if err := manager.Backend.Exec(prof.Name, win.Name, "", pane.Command); err != nil {
						logInstance.Warning(fmt.Sprintf("Failed to execute command: %v", err))
					}
				}
			}
		}

		logInstance.Info(fmt.Sprintf("Profile '%s' loaded successfully", prof.Name))
		return manager.Backend.AttachSession(prof.Name)
	},
}
