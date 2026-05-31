package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// --- discoverVMs ---

func TestDiscoverVMs(t *testing.T) {
	base := t.TempDir()

	for _, vm := range []string{"epsylon", "worker01"} {
		dir := filepath.Join(base, vm)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, vm+".sock"))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	sockets, err := discoverVMs(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sockets) != 2 {
		t.Fatalf("expected 2 sockets, got %d", len(sockets))
	}
	for _, name := range []string{"epsylon", "worker01"} {
		if _, ok := sockets[name]; !ok {
			t.Errorf("expected socket for %s", name)
		}
	}
}

func TestDiscoverVMsEmpty(t *testing.T) {
	sockets, err := discoverVMs(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sockets) != 0 {
		t.Errorf("expected 0 sockets, got %d", len(sockets))
	}
}

func TestDiscoverVMsNotFound(t *testing.T) {
	_, err := discoverVMs("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- cgroup helpers ---

func writeCgroupFile(t *testing.T, base, vmName, file, content string) {
	t.Helper()
	dir := filepath.Join(base, "microvm@"+vmName+".service")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCgroupMemory(t *testing.T) {
	base := t.TempDir()
	writeCgroupFile(t, base, "epsylon", "memory.current", "6576369664\n")

	mem, err := cgroupMemory(base, "epsylon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mem != 6576369664 {
		t.Errorf("expected 6576369664, got %d", mem)
	}
}

func TestCgroupMemoryNotFound(t *testing.T) {
	_, err := cgroupMemory(t.TempDir(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCgroupCPU(t *testing.T) {
	base := t.TempDir()
	writeCgroupFile(t, base, "epsylon", "cpu.stat",
		"usage_usec 197018549308\nuser_usec 120800433616\nsystem_usec 76218115692\nnice_usec 0\n")

	stats, err := cgroupCPU(base, "epsylon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats["usage_usec"] != 197018549308 {
		t.Errorf("expected 197018549308, got %d", stats["usage_usec"])
	}
	if stats["user_usec"] != 120800433616 {
		t.Errorf("expected 120800433616, got %d", stats["user_usec"])
	}
	if stats["system_usec"] != 76218115692 {
		t.Errorf("expected 76218115692, got %d", stats["system_usec"])
	}
}

// --- server ---

type mockVMExecutor struct {
	result json.RawMessage
	err    error
	vms    []string
}

func (m *mockVMExecutor) Execute(vmName, method string, args any) (json.RawMessage, error) {
	return m.result, m.err
}

func (m *mockVMExecutor) List() []string {
	return m.vms
}

func (m *mockVMExecutor) Metrics() map[string]VMMetricsSample { return nil }

func (m *mockVMExecutor) GetAddr(vmName string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "192.168.0.100", nil
}

func serverPipe(t *testing.T, exec vmExecutor) net.Conn {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	s := &Server{vms: exec}
	go s.handle(server)
	return client
}

func TestServerHandleSuccess(t *testing.T) {
	client := serverPipe(t, &mockVMExecutor{
		result: json.RawMessage(`{"running":true,"status":"running"}`),
	})

	enc := json.NewEncoder(client)
	dec := json.NewDecoder(client)

	_ = enc.Encode(agentRequest{VM: "epsylon", Method: "query-status"})

	var resp agentResponse
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error in response: %s", resp.Error)
	}
	if string(resp.Result) != `{"running":true,"status":"running"}` {
		t.Errorf("unexpected result: %s", resp.Result)
	}
}

func TestServerHandleGetAddr(t *testing.T) {
	client := serverPipe(t, &mockVMExecutor{})

	enc := json.NewEncoder(client)
	dec := json.NewDecoder(client)

	_ = enc.Encode(agentRequest{VM: "epsylon", Method: "get-addr"})

	var resp agentResponse
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
	var addr string
	if err := json.Unmarshal(resp.Result, &addr); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if addr != "192.168.0.100" {
		t.Errorf("unexpected addr: %s", addr)
	}
}

func TestServerHandleListVMs(t *testing.T) {
	client := serverPipe(t, &mockVMExecutor{vms: []string{"epsylon", "worker03"}})

	enc := json.NewEncoder(client)
	dec := json.NewDecoder(client)

	_ = enc.Encode(agentRequest{Method: "list-vms"})

	var resp agentResponse
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}

	var vms []string
	if err := json.Unmarshal(resp.Result, &vms); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if len(vms) != 2 || vms[0] != "epsylon" || vms[1] != "worker03" {
		t.Errorf("unexpected vms: %v", vms)
	}
}

func TestServerHandleExecuteError(t *testing.T) {
	client := serverPipe(t, &mockVMExecutor{
		err: os.ErrNotExist,
	})

	enc := json.NewEncoder(client)
	dec := json.NewDecoder(client)

	_ = enc.Encode(agentRequest{VM: "unknown", Method: "query-status"})

	var resp agentResponse
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected error in response, got none")
	}
}
