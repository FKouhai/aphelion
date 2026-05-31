package connect

import (
	"context"
	"encoding/json"
	"net"
	"testing"
)

type stubStatus struct {
	State string `json:"state"`
}

func TestParse(t *testing.T) {
	t.Run("valid json", func(t *testing.T) {
		raw := json.RawMessage(`{"state":"running"}`)
		s, err := Parse[stubStatus](raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.State != "running" {
			t.Errorf("expected running, got %s", s.State)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := Parse[stubStatus](json.RawMessage(`{invalid`))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		type other struct{ Count int }
		raw := json.RawMessage(`{"state":"running"}`)
		o, err := Parse[other](raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if o.Count != 0 {
			t.Errorf("expected zero value, got %d", o.Count)
		}
	})
}

func agentPipe(t *testing.T) (*AgentClient, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return newAgentClient(client), server
}

func TestExecute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		agent, server := agentPipe(t)

		go func() {
			dec := json.NewDecoder(server)
			enc := json.NewEncoder(server)
			var req agentRequest
			if err := dec.Decode(&req); err != nil {
				return
			}
			_ = enc.Encode(agentResponse{Result: json.RawMessage(`{"state":"running"}`)})
		}()

		raw, err := agent.Execute(context.Background(), "epsylon", "query-status", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s, err := Parse[stubStatus](raw)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if s.State != "running" {
			t.Errorf("expected running, got %s", s.State)
		}
	})

	t.Run("agent returns error", func(t *testing.T) {
		agent, server := agentPipe(t)

		go func() {
			dec := json.NewDecoder(server)
			enc := json.NewEncoder(server)
			var req agentRequest
			if err := dec.Decode(&req); err != nil {
				return
			}
			_ = enc.Encode(agentResponse{Error: "vm not found"})
		}()

		_, err := agent.Execute(context.Background(), "epsylon", "query-status", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("malformed response", func(t *testing.T) {
		agent, server := agentPipe(t)

		go func() {
			dec := json.NewDecoder(server)
			var req agentRequest
			_ = dec.Decode(&req)
			_, _ = server.Write([]byte(`{invalid json`))
			server.Close()
		}()

		_, err := agent.Execute(context.Background(), "epsylon", "query-status", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestAgentClientClose(t *testing.T) {
	agent, _ := agentPipe(t)
	if err := agent.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}
