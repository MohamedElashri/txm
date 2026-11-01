package cmd

import (
	"fmt"
	localmcp "github.com/MohamedElashri/txm/pkg/mcp"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "licenses", Short: "Print retained license notices for dependencies introduced by MCP", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if jsonFlag {
			return extensionResult(map[string]string{"third_party_notices": localmcp.ThirdPartyNotices}, nil)
		}
		_, e := fmt.Fprint(cmd.OutOrStdout(), localmcp.ThirdPartyNotices)
		return e
	}})
}
