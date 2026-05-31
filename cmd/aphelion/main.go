package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aphelion",
	Short: "TUI for managing microvm.nix VMs across NixOS hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		// launch TUI when no subcommand is given
		return runTUI()
	},
}

var (
	cfgFile   string
	agentPort int
)

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/aphelion/config.yaml)")
	rootCmd.PersistentFlags().IntVar(&agentPort, "agent-port", 7373, "port the aphelion-agent listens on")

	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(lifecycleCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
