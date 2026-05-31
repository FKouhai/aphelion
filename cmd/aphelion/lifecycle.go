package main

import (
	"context"
	"fmt"

	"aphelion/pkg/connect"
	"aphelion/pkg/qmp"
	"github.com/spf13/cobra"
)

var lifecycleCmd = &cobra.Command{
	Use:   "vm",
	Short: "VM lifecycle commands",
}

func init() {
	lifecycleCmd.AddCommand(restartCmd, stopCmd, resumeCmd)
}

var restartCmd = &cobra.Command{
	Use:   "restart <host> <vm>",
	Short: "Restart a VM",
	Args:  cobra.ExactArgs(2),
	RunE:  runLifecycle("restart"),
}

var stopCmd = &cobra.Command{
	Use:   "stop <host> <vm>",
	Short: "Stop a VM",
	Args:  cobra.ExactArgs(2),
	RunE:  runLifecycle("stop"),
}

var resumeCmd = &cobra.Command{
	Use:   "resume <host> <vm>",
	Short: "Resume a stopped VM",
	Args:  cobra.ExactArgs(2),
	RunE:  runLifecycle("resume"),
}

func runLifecycle(action string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		hostName, vmName := args[0], args[1]

		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		host, err := cfg.ByName(hostName)
		if err != nil {
			return err
		}

		conn, err := connect.Dial(*host)
		if err != nil {
			return fmt.Errorf("connecting to %s: %w", hostName, err)
		}
		defer conn.Close()

		agent, err := conn.Agent(agentPort)
		if err != nil {
			return err
		}
		defer agent.Close()

		client := qmp.New(agent, vmName)
		ctx := context.Background()

		switch action {
		case "restart":
			err = client.Reset(ctx)
		case "stop":
			err = client.Stop(ctx)
		case "resume":
			err = client.Resume(ctx)
		}

		if err != nil {
			return fmt.Errorf("%s %s: %w", action, vmName, err)
		}

		fmt.Printf("%s: %s ok\n", vmName, action)
		return nil
	}
}
