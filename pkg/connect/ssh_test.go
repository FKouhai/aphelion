package connect

import (
	"io"
	"testing"
)

func TestSSHSessionRead(t *testing.T) {
	pr, pw := io.Pipe()
	s := &sshSession{stdout: pr}

	want := []byte("hello from vm")
	go func() {
		_, _ = pw.Write(want)
	}()

	buf := make([]byte, len(want))
	n, err := s.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf[:n]) != string(want) {
		t.Errorf("expected %q, got %q", want, buf[:n])
	}
}

func TestSSHSessionWrite(t *testing.T) {
	pr, pw := io.Pipe()
	s := &sshSession{stdin: pw}

	want := []byte("command")
	go func() {
		_, _ = s.Write(want)
	}()

	buf := make([]byte, len(want))
	n, err := pr.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf[:n]) != string(want) {
		t.Errorf("expected %q, got %q", want, buf[:n])
	}
}
