package qmp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type mockExecutor struct {
	result json.RawMessage
	err    error
	method string
}

func (m *mockExecutor) Execute(_ context.Context, method string, _ any) (json.RawMessage, error) {
	m.method = method
	return m.result, m.err
}

func newClient(raw json.RawMessage, err error) (*Client, *mockExecutor) {
	m := &mockExecutor{result: raw, err: err}
	return &Client{agent: m}, m
}

var errAgent = errors.New("agent error")

func TestQueryStatus(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		c, m := newClient(json.RawMessage(`{"running":true,"singlestep":false,"status":"running"}`), nil)
		s, err := c.QueryStatus(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Status != RunStateRunning {
			t.Errorf("expected running, got %s", s.Status)
		}
		if m.method != cmdQueryStatus {
			t.Errorf("expected %s, got %s", cmdQueryStatus, m.method)
		}
	})

	t.Run("agent error", func(t *testing.T) {
		c, _ := newClient(nil, errAgent)
		_, err := c.QueryStatus(context.Background())
		if !errors.Is(err, errAgent) {
			t.Errorf("expected errAgent, got %v", err)
		}
	})
}

func TestQueryMemory(t *testing.T) {
	c, _ := newClient(json.RawMessage(`{"base-memory":2147483648,"plugged-memory":0}`), nil)
	m, err := c.QueryMemory(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.BaseMemory != 2147483648 {
		t.Errorf("expected 2147483648, got %d", m.BaseMemory)
	}
}

func TestQueryCPUs(t *testing.T) {
	c, _ := newClient(json.RawMessage(`[{"cpu-index":0,"thread-id":123,"target":"x86_64","qom-path":"/machine/unattached/device[0]"}]`), nil)
	cpus, err := c.QueryCPUs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cpus) != 1 {
		t.Fatalf("expected 1 cpu, got %d", len(cpus))
	}
	if cpus[0].Index != 0 {
		t.Errorf("expected index 0, got %d", cpus[0].Index)
	}
}

func TestQueryBlockStats(t *testing.T) {
	c, _ := newClient(json.RawMessage(`[{"device":"virtio0","node-name":"drive0","stats":{"rd_bytes":1024,"wr_bytes":2048,"rd_operations":10,"wr_operations":20}}]`), nil)
	stats, err := c.QueryBlockStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].Stats.ReadBytes != 1024 {
		t.Errorf("expected 1024, got %d", stats[0].Stats.ReadBytes)
	}
}

func TestQueryCharDev(t *testing.T) {
	c, _ := newClient(json.RawMessage(`[{"label":"serial0","filename":"/var/lib/microvms/epsylon/serial.sock","frontend-open":true}]`), nil)
	devs, err := c.QueryCharDev(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devs) != 1 {
		t.Fatalf("expected 1 chardev, got %d", len(devs))
	}
	if devs[0].Label != "serial0" {
		t.Errorf("expected serial0, got %s", devs[0].Label)
	}
}

func TestLifecycle(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(*Client) error
		wantCmd string
	}{
		{"reset", func(c *Client) error { return c.Reset(context.Background()) }, cmdReset},
		{"powerdown", func(c *Client) error { return c.PowerDown(context.Background()) }, cmdPowerdown},
		{"stop", func(c *Client) error { return c.Stop(context.Background()) }, cmdStop},
		{"resume", func(c *Client) error { return c.Resume(context.Background()) }, cmdResume},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, m := newClient(json.RawMessage(`{}`), nil)
			if err := tt.fn(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.method != tt.wantCmd {
				t.Errorf("expected %s, got %s", tt.wantCmd, m.method)
			}
		})
	}
}
