package cmd

import (
	"fmt"
)

// parseSessionArgs extracts and validates the session name from arguments
func parseSessionArgs(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("missing session name")
	}
	session := getSessionName(args[0])
	if err := validateName(session); err != nil {
		return "", err
	}
	return session, nil
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
