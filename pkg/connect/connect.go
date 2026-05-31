package connect

import (
	"aphelion/pkg/config"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// HostConn binds a particular host with its client
type HostConn struct {
	client *ssh.Client
	host   config.Host
}

// Session transport interface
type Session interface {
	io.ReadWriter
	Close() error
	Resize(rows, cols uint16) error
}

// Dial wraps an ssh client per host
func Dial(h config.Host) (*HostConn, error) {
	cfg := &ssh.ClientConfig{
		User:            h.Username,
		Auth:            authMethods(h.DisplayName),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	port := 22
	if h.Port != nil {
		port = *h.Port
	}
	c, err := ssh.Dial("tcp", net.JoinHostPort(h.Address, strconv.Itoa(port)), cfg)
	if err != nil {
		return nil, err
	}

	return &HostConn{
		client: c,
		host:   h,
	}, nil
}

func (c *HostConn) Close() error {
	return c.client.Close()
}

func authMethods(displayName string) []ssh.AuthMethod {
	var signers []ssh.Signer

	signers = append(signers, keyFileSigners()...)

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			a := agent.NewClient(conn)
			if agentSigners, err := a.Signers(); err == nil {
				signers = append(signers, agentSigners...)
			}
		}
	}

	var methods []ssh.AuthMethod
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}

	methods = append(methods, ssh.PasswordCallback(func() (string, error) {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return "", fmt.Errorf("opening tty: %w", err)
		}
		defer tty.Close()
		fmt.Fprintf(tty, "%s SSH password: ", displayName)
		passwd, err := term.ReadPassword(int(tty.Fd()))
		fmt.Fprintln(tty)
		return string(passwd), err
	}))

	return methods
}

func keyFileSigners() []ssh.Signer {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	entries, err := os.ReadDir(filepath.Join(home, ".ssh"))
	if err != nil {
		return nil
	}

	var signers []ssh.Signer
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}
		key, err := os.ReadFile(filepath.Join(home, ".ssh", entry.Name()))
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			continue // not a key or passphrase-protected
		}
		signers = append(signers, signer)
	}
	return signers
}
