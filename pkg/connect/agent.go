package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/coder/websocket"
)

type AgentClient struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

type agentRequest struct {
	VM     string `json:"vm"`
	Method string `json:"method"`
	Args   any    `json:"args,omitempty"`
}

type agentResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type LogReader struct {
	ctx  context.Context
	conn *websocket.Conn
	buf  []byte
}

func (l *LogReader) Read(b []byte) (int, error) {
	for len(l.buf) == 0 {
		_, msg, err := l.conn.Read(l.ctx)
		if err != nil {
			return 0, err
		}
		l.buf = append(msg, '\n')
	}
	n := copy(b, l.buf)
	l.buf = l.buf[n:]
	return n, nil
}

func (l *LogReader) Close() error {
	return l.conn.Close(websocket.StatusNormalClosure, "")
}

// Parse generic method used to parse the raw messages
func Parse[T any](raw json.RawMessage) (T, error) {
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, fmt.Errorf("parsing response: %w", err)
	}
	return result, nil
}

func newAgentClient(conn net.Conn) *AgentClient {
	return &AgentClient{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn),
	}
}

// Agent port forward to local tcp port
func (c *HostConn) Agent(port int) (*AgentClient, error) {
	conn, err := c.client.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, fmt.Errorf("forwarding to agent on %s: %w", c.host.DisplayName, err)
	}
	return newAgentClient(conn), nil
}

// Execute encodes the request, decodes the req and return the final result
func (a *AgentClient) Execute(ctx context.Context, vmName, method string, args any) (json.RawMessage, error) {
	if err := a.enc.Encode(agentRequest{VM: vmName, Method: method, Args: args}); err != nil {
		return nil, fmt.Errorf("sending command: %w", err)
	}
	var resp agentResponse
	if err := a.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("agent: %s", resp.Error)
	}
	return resp.Result, nil
}

// Close closes the connection
func (a *AgentClient) Close() error {
	return a.conn.Close()
}

func (a *AgentClient) VMAddr(ctx context.Context, vmName string) (string, error) {
	raw, err := a.Execute(ctx, vmName, "get-addr", nil)
	if err != nil {
		return "", err
	}
	return Parse[string](raw)
}

func (a *AgentClient) ListVMs(ctx context.Context) ([]string, error) {
	raw, err := a.Execute(ctx, "", "list-vms", nil)
	if err != nil {
		return nil, err
	}
	return Parse[[]string](raw)
}

// VMMetricsSample holds the latest collected metrics for one VM.
type VMMetricsSample struct {
	Up          bool    `json:"up"`
	MemoryBytes uint64  `json:"memory_bytes"`
	CPUPercent  float64 `json:"cpu_percent"`
}

func (a *AgentClient) Metrics(ctx context.Context) (map[string]VMMetricsSample, error) {
	raw, err := a.Execute(ctx, "", "get-metrics", nil)
	if err != nil {
		return nil, err
	}
	return Parse[map[string]VMMetricsSample](raw)
}

func (a *AgentClient) Logs(ctx context.Context, vmName string, logdPort int) (*LogReader, error) {
	ip, err := a.VMAddr(ctx, vmName)
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://%s:%d/logs", ip, logdPort), nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to aphelion-logd on %s: %w", vmName, err)
	}
	return &LogReader{ctx: ctx, conn: conn}, nil
}

// Console represents the machine console for the session
func (a *AgentClient) Console(ctx context.Context, vmName string) (Session, error) {
	return nil, fmt.Errorf("not implemented")
}
