package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [binary_path]",
	Short: "Push a pre-built binary to NX Sandbox",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("push requested for binary: %s\n", args[0])
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)
}
