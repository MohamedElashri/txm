package cmd

import (
	"fmt"
	"os"
)

func scopedEnvironment(key, value string) (func(), error) {
	old, present := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		return nil, err
	}
	return func() {
		if present {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	}, nil
}

// parseSessionArgs extracts and validates the session name from arguments
func parseSessionArgs(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("missing session name")
	}
	return sessionTarget(args[0])
}

// parseWindowArgs extracts and validates the session and window name from arguments
func parseWindowArgs(args []string) (string, string, error) {
	session, err := parseSessionArgs(args)
	if err != nil {
		return "", "", err
	}
	if len(args) < 2 {
		return session, "", fmt.Errorf("missing window name")
	}
	window := args[1]
	return session, window, nil
}

// parsePaneArgs extracts and validates the session, window, and pane ID from arguments
func parsePaneArgs(args []string) (string, string, string, error) {
	session, window, err := parseWindowArgs(args)
	if err != nil {
		return "", "", "", err
	}
	if len(args) < 3 {
		return session, window, "", fmt.Errorf("missing pane id")
	}
	pane := args[2]
	return session, window, pane, nil
}
