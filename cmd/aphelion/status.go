package main

import (
	"context"
	"fmt"

	"aphelion/pkg/config"
	"aphelion/pkg/connect"
	"aphelion/pkg/qmp"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List all VMs and their current state across all hosts",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	fmt.Printf("%-16s %-16s %-10s\n", "HOST", "VM", "STATE")
	fmt.Printf("%-16s %-16s %-10s\n", "----", "--", "-----")

	for _, host := range cfg.Hosts {
		if err := statusForHost(host); err != nil {
			fmt.Printf("%-16s %-16s %s\n", host.DisplayName, "-", fmt.Sprintf("error: %v", err))
		}
	}
	return nil
}

func statusForHost(host config.Host) error {
	conn, err := connect.Dial(host)
	if err != nil {
		return err
	}
	defer conn.Close()

	agent, err := conn.Agent(agentPort)
	if err != nil {
		return err
	}
	defer agent.Close()

	vms, err := discoverRemoteVMs(agent, host)
	if err != nil {
		return err
	}

	for _, vmName := range vms {
		client := qmp.New(agent, vmName)
		status, err := client.QueryStatus(context.Background())
		if err != nil {
			fmt.Printf("%-16s %-16s %s\n", host.DisplayName, vmName, "unknown")
			continue
		}
		fmt.Printf("%-16s %-16s %-10s\n", host.DisplayName, vmName, status.Status)
	}
	return nil
}
