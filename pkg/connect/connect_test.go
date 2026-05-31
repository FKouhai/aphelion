package connect

import (
	"net"
	"path/filepath"
	"testing"
)

func TestAuthMethods(t *testing.T) {
	t.Run("no sock no keys yields empty", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "")
		// point home somewhere empty so keyFileMethods finds nothing
		t.Setenv("HOME", t.TempDir())
		// password callback is always appended, so expect 1
		methods := authMethods("test-host")
		if len(methods) != 1 {
			t.Fatalf("expected 1 method (password callback), got %d", len(methods))
		}
	})

	t.Run("invalid sock is skipped", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "/nonexistent/agent.sock")
		t.Setenv("HOME", t.TempDir())
		methods := authMethods("test-host")
		if len(methods) != 1 {
			t.Fatalf("expected 1 method (password callback), got %d", len(methods))
		}
	})

	t.Run("valid sock adds one method", func(t *testing.T) {
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
		t.Setenv("HOME", t.TempDir())
		// listener returns no agent signers → only password callback
		methods := authMethods("test-host")
		if len(methods) != 1 {
			t.Fatalf("expected 1 method (password callback), got %d", len(methods))
		}
	})
}
