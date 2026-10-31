package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/spf13/cobra"
)

func isExtension(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c == taskCmd || c == remoteCmd {
			return true
		}
	}
	return false
}
func extensionResult(data any, e error) error {
	if !jsonFlag && data != nil {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(core.Plain(b))
	}
	failures := []*core.OperationError{}
	if e != nil {
		failures = append(failures, core.Normalize(e))
	}
	success := 1
	if data == nil {
		success = 0
	}
	return queryResult(data, failures, success)
}
