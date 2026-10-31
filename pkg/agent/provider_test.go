package agent

import "testing"

func TestAutomaticClaudeSettingsNeverShadowExplicitSettings(t *testing.T) {
	for _, argv := range [][]string{nil, {"claude", "--settings", "private.json"}, {"claude", "--settings={}"}} {
		if ValidateAutomaticClaudeArgs(argv) == nil {
			t.Fatal("ambiguous explicit settings accepted", argv)
		}
	}
	for _, argv := range [][]string{{"claude", "--init-only"}, {"claude", "--setting-sources", "user,project"}, {"claude", "--", "--settings"}} {
		if err := ValidateAutomaticClaudeArgs(argv); err != nil {
			t.Fatal(err)
		}
	}
}
