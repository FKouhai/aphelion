package connect

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
)

type sshSession struct {
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

// OpenSSH instantiates an ssh tunnel
func (c *HostConn) OpenSSH(vmAddr, username string) (Session, error) {
	tunn, err := c.client.Dial("tcp", vmAddr)

	if err != nil {
		return nil, err
	}

	sshClientConn, chans, reqs, err := ssh.NewClientConn(tunn, vmAddr, &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{c.auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		_ = tunn.Close()
		return nil, fmt.Errorf("ssh handshake with %s: %w", vmAddr, err)
	}
	vmClient := ssh.NewClient(sshClientConn, chans, reqs)
	sess, err := newSSHSession(vmClient)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func newSSHSession(client *ssh.Client) (*sshSession, error) {
	s, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	stdin, err := s.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := s.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{}); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("requesting pty: %w", err)
	}

	if err := s.Shell(); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("starting shell %w", err)
	}
	return &sshSession{session: s, stdin: stdin, stdout: stdout}, nil
}

func (s *sshSession) Read(p []byte) (int, error)  { return s.stdout.Read(p) }
func (s *sshSession) Write(p []byte) (int, error) { return s.stdin.Write(p) }
func (s *sshSession) Close() error {
	_ = s.stdin.Close()
	return s.session.Close()
}

func (s *sshSession) Resize(rows, cols uint16) error {
	return s.session.WindowChange(int(rows), int(cols))
}
