package main

import (
	"fmt"
	"os"

	"github.com/NethermindEth/chaoschain-launchpad/cmd/agent/commands"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "agent-cli",
	Short: "ChaosChain Agent CLI",
	Long:  `Command line interface for managing ChaosChain agents.`,
}

func init() {
	// Add commands
	rootCmd.AddCommand(commands.CreateCmd)
	rootCmd.AddCommand(commands.ListCmd)
	rootCmd.AddCommand(commands.TemplateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
} 