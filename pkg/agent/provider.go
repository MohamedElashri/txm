package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func ProviderConfig(executable string, c Credential) ([]byte, error) {
	if !filepath.IsAbs(executable) || !runtimepath.ValidID(c.AgentID) || !runtimepath.ValidID(c.Generation) || (c.Provider != "claude" && c.Provider != "codex") {
		return nil, core.Failure("unsupported", "provider configuration unavailable")
	}
	command := "XDG_STATE_HOME=" + shellQuote(c.Namespace.StateHome) + " XDG_RUNTIME_DIR=" + shellQuote(c.Namespace.RuntimeDir) + " TMPDIR=" + shellQuote(c.Namespace.TempDir) + " " + shellQuote(executable) + " agent hook " + c.AgentID + " --generation " + c.Generation + " --provider " + c.Provider
	type action struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type match struct {
		Hooks []action `json:"hooks"`
	}
	hooks := map[string][]match{}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PermissionRequest", "Stop"} {
		hooks[event] = []match{{[]action{{"command", command}}}}
	}
	return json.MarshalIndent(struct {
		Hooks map[string][]match `json:"hooks"`
	}{hooks}, "", "  ")
}
