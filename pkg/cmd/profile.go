package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/config"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/logger"
	"github.com/MohamedElashri/txm/pkg/profile"
	"github.com/spf13/cobra"
)

var profileDryRun, profileDetach, profileAttachExisting, profileReadOnly bool
var profileCmd = &cobra.Command{
	Use: "profile PATH", Short: "Validate and apply a declarative profile", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		readOnly := profileReadOnly || os.Getenv("TXM_READ_ONLY") == "1"
		if readOnly && (!profileAttachExisting || profileDetach) {
			return core.Failure("invalid_usage", "read-only profile requires --attach-existing and cannot use --detach")
		}
		spec, e := config.LoadProfile(args[0])
		if e != nil {
			return e
		}
		strict := strings.ToLower(backendFlag)
		if spec.Backend != "" {
			chosen, e := config.ParseBackend(spec.Backend)
			if e != nil {
				return core.Failure("invalid_profile", "invalid profile backend")
			}
			if strict != "" && strict != string(chosen) {
				return core.Failure("invalid_profile", "profile backend conflicts with --backend")
			}
			strict = string(chosen)
		}
		cfg, e := config.LoadConfig()
		if e != nil {
			result := core.Failure("invalid_config", "could not load valid configuration")
			result.Cause = e
			return result
		}
		var b backend.TerminalMultiplexer
		if strict != "" {
			b, e = backend.NewBackend(strict, cmd.Context())
			if e != nil {
				return e
			}
		} else {
			b = backend.NewManager(cfg, logger.NewLogger(verbosity), cmd.Context()).Backend
		}
		cwd, e := os.Getwd()
		if e != nil {
			return e
		}
		plan, e := profile.Compile(spec, b.Name(), os.Getenv("TXM_SESSION_PREFIX"), cwd, os.Getenv("SHELL"))
		if e != nil {
			return e
		}
		if !b.IsAvailable() {
			return core.Scope(core.Failure("backend_unavailable", "profile backend unavailable"), b.Name())
		}
		if e = backend.PreflightProfile(cmd.Context(), b); e != nil {
			return e
		}
		if profileDryRun {
			fmt.Printf("Profile plan: %s/%s\n", plan.Backend, plan.Name)
			caps, e := b.BackendCapabilities(cmd.Context())
			if e != nil {
				return e
			}
			keys := []string{"session_lifecycle", "working_directory", "input_injection"}
			if b.Name() == "tmux" {
				keys = append(keys, "exact_pane_targeting", "window_layout")
			}
			for _, key := range keys {
				fmt.Printf("  capability %s: %s\n", key, caps.Capabilities[key].Level)
			}
			fmt.Println("Dry run: no session discovery, creation or attachment")
			policy := "conflict"
			if profileAttachExisting {
				policy = "attach unchanged"
			}
			if readOnly {
				policy = "observe existing only"
			}
			fmt.Printf("Existing session: %s\n", policy)
			for i, w := range plan.Windows {
				layout := w.Layout
				if layout == "" {
					layout = "backend default"
				}
				fmt.Printf("  window %d: %s, layout %s\n", i+1, w.Name, layout)
				for j, p := range w.Panes {
					command := "none"
					if p.Command != "" {
						command = "terminal input (omitted)"
					}
					fmt.Printf("    pane %d: cwd=%q split=%q command=%s\n", j+1, p.Dir, p.Split, command)
				}
			}
			return nil
		}
		found, e := b.Lookup(cmd.Context(), plan.Name)
		if e == nil {
			if !profileAttachExisting {
				return core.Scope(core.Failure("conflict", "profile session already exists; use --attach-existing to attach unchanged"), b.Name())
			}
			if profileDetach {
				fmt.Printf("Profile session unchanged: %s/%s\n", b.Name(), plan.Name)
				return nil
			}
			target := found.Locator.Name
			if b.Name() == "tmux" && found.BackendID != nil {
				target = *found.BackendID
			}
			if readOnly {
				observer, ok := b.(interface{ AttachReadOnly(string) error })
				if !ok {
					return core.Failure("unsupported", "read-only attach unsupported")
				}
				return observer.AttachReadOnly(target)
			}
			return b.AttachSession(target)
		}
		if core.Normalize(e).Code != "not_found" {
			return e
		}
		if readOnly {
			return core.Scope(core.Failure("not_found", "profile session does not exist; read-only never creates it"), b.Name())
		}
		owned, e := backend.ApplyProfile(cmd.Context(), b, plan)
		if e != nil {
			return e
		}
		if !profileDetach {
			if e = backend.AttachProfile(cmd.Context(), b, owned, false); e != nil {
				result := core.Scope(core.Failure(core.Normalize(e).Code, fmt.Sprintf("profile applied, but attachment failed; inspect %s/%s for remaining resources", b.Name(), plan.Name)), b.Name())
				result.Cause = e
				return result
			}
		}
		fmt.Printf("Profile applied: %s/%s\n", plan.Backend, plan.Name)
		return nil
	},
}

func init() {
	profileCmd.Flags().BoolVar(&profileDryRun, "dry-run", false, "Validate and show targets/cwd without applying or exposing commands")
	profileCmd.Flags().BoolVar(&profileDetach, "detach", false, "Apply and leave the session detached")
	profileCmd.Flags().BoolVar(&profileAttachExisting, "attach-existing", false, "Attach unchanged when the exact profile session already exists")
	profileCmd.Flags().BoolVarP(&profileReadOnly, "read-only", "r", false, "Observe an existing session only; requires --attach-existing")
}
