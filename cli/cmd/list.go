package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("list requested")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
