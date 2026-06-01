package main

import (
	"fmt"
	"os"

	"aphelion/pkg/version"

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
	logdPort  int
)

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/aphelion/config.yaml)")
	rootCmd.PersistentFlags().IntVar(&agentPort, "agent-port", 7373, "port the aphelion-agent listens on")
	rootCmd.PersistentFlags().IntVar(&logdPort, "logd-port", 7374, "port the aphelion-logd listens on")

	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(lifecycleCmd)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println(version.Version) },
	})
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
