package main

import (
	"fmt"
	"io"
	"os"

	"aphelion/pkg/connect"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh <host> <vm-addr>",
	Short: "Open an SSH session to a VM through its host",
	Args:  cobra.ExactArgs(2),
	RunE:  runSSH,
}

var sshUser string

func init() {
	sshCmd.Flags().StringVar(&sshUser, "user", "", "username for the VM SSH session (defaults to host username)")
}

func runSSH(cmd *cobra.Command, args []string) error {
	hostName, vmAddr := args[0], args[1]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	host, err := cfg.ByName(hostName)
	if err != nil {
		return err
	}

	username := host.Username
	if sshUser != "" {
		username = sshUser
	} else if host.VMUsername != "" {
		username = host.VMUsername
	}

	conn, err := connect.Dial(*host)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", hostName, err)
	}
	defer conn.Close()

	session, err := conn.OpenSSH(vmAddr, username)
	if err != nil {
		return fmt.Errorf("opening ssh to %s: %w", vmAddr, err)
	}
	defer session.Close()

	go io.Copy(session, os.Stdin)
	io.Copy(os.Stdout, session)
	return nil
}
