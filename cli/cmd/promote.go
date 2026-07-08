package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var promoteCmd = &cobra.Command{
	Use:   "promote [app_slug]",
	Short: "Promote latest preview deployment to production",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("promote requested for app: %s\n", args[0])
	},
}

func init() {
	rootCmd.AddCommand(promoteCmd)
}
