package connect

import (
	"net"
	"path/filepath"
	"testing"
)

func TestAgentAuth(t *testing.T) {
	t.Run("SSH_AUTH_SOCK not set", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "")
		_, err := agentAuth()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid socket path", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "/nonexistent/agent.sock")
		_, err := agentAuth()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("valid socket", func(t *testing.T) {
		sock := filepath.Join(t.TempDir(), "agent.sock")
		l, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				conn.Close()
			}
		}()

		t.Setenv("SSH_AUTH_SOCK", sock)
		auth, err := agentAuth()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if auth == nil {
			t.Fatal("expected non-nil auth method")
		}
	})
}
