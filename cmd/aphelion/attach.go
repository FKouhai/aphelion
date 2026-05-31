package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"

	"aphelion/pkg/config"
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

	return doAttach(*host, vmName, attachUser, attachPort)
}

// doAttach is used by the CLI attach subcommand.
func doAttach(host config.Host, vmName, user, port string) error {
	user, port = resolveAttachParams(host, user, port)

	conn, err := connect.Dial(host)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", host.DisplayName, err)
	}
	defer conn.Close()

	ag, err := conn.Agent(agentPort)
	if err != nil {
		return err
	}
	defer ag.Close()

	ip, err := ag.VMAddr(context.Background(), vmName)
	if err != nil {
		return fmt.Errorf("resolving address for %s: %w", vmName, err)
	}

	fd := int(os.Stdin.Fd())
	w, h, err := term.GetSize(fd)
	if err != nil {
		w, h = 80, 24
	}

	session, err := conn.OpenSSH(net.JoinHostPort(ip, port), user, w, h)
	if err != nil {
		return fmt.Errorf("opening ssh to %s (%s): %w", vmName, ip, err)
	}
	defer session.Close()

	return runSession(session, os.Stdin, os.Stdout, fd)
}

// sshExecCmd implements tea.ExecCommand so the TUI can suspend, attach, then resume.
type sshExecCmd struct {
	host   config.Host
	vmName string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (c *sshExecCmd) SetStdin(r io.Reader)  { c.stdin = r }
func (c *sshExecCmd) SetStdout(w io.Writer) { c.stdout = w }
func (c *sshExecCmd) SetStderr(w io.Writer) { c.stderr = w }

func (c *sshExecCmd) Run() error {
	user, port := resolveAttachParams(c.host, "", "")

	conn, err := connect.Dial(c.host)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", c.host.DisplayName, err)
	}
	defer conn.Close()

	ag, err := conn.Agent(agentPort)
	if err != nil {
		return err
	}
	defer ag.Close()

	ip, err := ag.VMAddr(context.Background(), c.vmName)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", c.vmName, err)
	}

	fd := int(os.Stdin.Fd())
	w, h, err := term.GetSize(fd)
	if err != nil {
		w, h = 80, 24
	}

	session, err := conn.OpenSSH(net.JoinHostPort(ip, port), user, w, h)
	if err != nil {
		return fmt.Errorf("ssh to %s: %w", c.vmName, err)
	}
	defer session.Close()

	return runSession(session, c.stdin, c.stdout, fd)
}

func newAttachExec(host config.Host, vmName string) tea.ExecCommand {
	return &sshExecCmd{host: host, vmName: vmName}
}

// runSession copies stdin/stdout and handles raw mode + terminal resize.
func runSession(session connect.Session, stdin io.Reader, stdout io.Writer, fd int) error {
	cr, err := cancelreader.NewReader(stdin)
	if err != nil {
		return fmt.Errorf("stdin reader: %w", err)
	}

	if term.IsTerminal(fd) {
		if old, err := term.MakeRaw(fd); err == nil {
			defer term.Restore(fd, old)
		}
	}

	// Forward terminal resize events until the session ends.
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigwinch:
				if w, h, err := term.GetSize(fd); err == nil {
					_ = session.Resize(uint16(h), uint16(w))
				}
			}
		}
	}()

	// Copy stdin → session via the cancel reader so it can be cleanly
	// interrupted when the session output ends, preventing a race with
	// bubbletea's input reader after the exec returns.
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(session, cr)
	}()

	defer func() {
		cancel()
		cr.Cancel()
		signal.Stop(sigwinch)
		wg.Wait()
	}()

	// Block until session output ends.
	io.Copy(stdout, session)

	return nil
}

func resolveAttachParams(host config.Host, user, port string) (string, string) {
	if user == "" {
		if host.VMUsername != "" {
			user = host.VMUsername
		} else {
			user = host.Username
		}
	}
	if port == "" {
		port = "22"
	}
	return user, port
}
