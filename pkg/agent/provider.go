package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

// Claude accepts a single explicit settings value. Prepending another flag
// would silently lose either the user's settings or the observation hooks.
// Preserve inherited sources, but require manual reviewed composition when
// the caller already supplied --settings.
func ValidateAutomaticClaudeArgs(argv []string) error {
	if len(argv) == 0 {
		return core.Failure("invalid_usage", "Claude executable required")
	}
	for _, arg := range argv[1:] {
		if arg == "--" {
			break
		}
		if arg == "--settings" || strings.HasPrefix(arg, "--settings=") {
			return core.Failure("unsupported", "automatic Claude hooks cannot replace explicit --settings; supervise without --provider and review hooks enable/config with the existing settings")
		}
	}
	return nil
}

func ProviderConfig(executable string, c Credential) ([]byte, error) {
	if !filepath.IsAbs(executable) || !runtimepath.ValidID(c.AgentID) || !runtimepath.ValidID(c.Generation) || (c.Provider != "claude" && c.Provider != "codex") {
		return nil, core.Failure("unsupported", "provider configuration unavailable")
	}
	command := "XDG_STATE_HOME=" + shellQuote(c.Namespace.StateHome) + " XDG_RUNTIME_DIR=" + shellQuote(c.Namespace.RuntimeDir) + " TMPDIR=" + shellQuote(c.Namespace.TempDir) + " " + shellQuote(executable) + " agent hook " + c.AgentID + " --generation " + c.Generation + " --provider " + c.Provider
	type action struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	type match struct {
		Hooks []action `json:"hooks"`
	}
	hooks := map[string][]match{}
	for _, event := range providerEventNames(c.Provider) {
		hooks[event] = []match{{[]action{{"command", command, 2}}}}
	}
	return json.MarshalIndent(struct {
		Hooks map[string][]match `json:"hooks"`
	}{hooks}, "", "  ")
}

func providerEventNames(provider string) []string {
	events := []string{"SessionStart", "UserPromptSubmit", "PermissionRequest", "PostToolUse", "Stop", "SessionEnd"}
	if provider == "claude" {
		events = append(events, "PostToolUseFailure", "StopFailure")
	}
	if provider == "codex" {
		events = append(events, "Interrupt")
	}
	return events
}
