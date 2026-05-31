package connect

import (
	"aphelion/pkg/config"
	"fmt"
	"io"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// HostConn binds a particular host with its client
type HostConn struct {
	auth   ssh.AuthMethod
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
	auth, err := agentAuth()
	if err != nil {
		return nil, err
	}

	cfg := &ssh.ClientConfig{
		User:            h.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	c, err := ssh.Dial("tcp", h.Address, cfg)
	if err != nil {
		return nil, err
	}

	return &HostConn{
		auth:   auth,
		client: c,
		host:   h,
	}, nil

}

func agentAuth() (ssh.AuthMethod, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("connecting to ssh agent: %w", err)
	}

	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), nil
}
