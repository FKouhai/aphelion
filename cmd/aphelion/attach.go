package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"aphelion/pkg/connect"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <host> <vm>",
	Short: "Open an SSH session to a VM by name through its host",
	Args:  cobra.ExactArgs(2),
	RunE:  runAttach,
}

var (
	attachUser string
	attachPort string
)

func init() {
	attachCmd.Flags().StringVar(&attachUser, "user", "", "username for the VM SSH session (defaults to host vm_username)")
	attachCmd.Flags().StringVar(&attachPort, "port", "22", "SSH port on the VM")
}

func runAttach(cmd *cobra.Command, args []string) error {
	hostName, vmName := args[0], args[1]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	host, err := cfg.ByName(hostName)
	if err != nil {
		return err
	}

	username := host.Username
	if attachUser != "" {
		username = attachUser
	} else if host.VMUsername != "" {
		username = host.VMUsername
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

	ip, err := agent.VMAddr(context.Background(), vmName)
	if err != nil {
		return fmt.Errorf("resolving address for %s: %w", vmName, err)
	}

	session, err := conn.OpenSSH(net.JoinHostPort(ip, attachPort), username)
	if err != nil {
		return fmt.Errorf("opening ssh to %s (%s): %w", vmName, ip, err)
	}
	defer session.Close()

	go io.Copy(session, os.Stdin)
	io.Copy(os.Stdout, session)
	return nil
}
