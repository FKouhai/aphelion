package qmp

import (
	"aphelion/pkg/connect"
	"context"
	"encoding/json"
)

type executor interface {
	Execute(ctx context.Context, vmName, method string, args any) (json.RawMessage, error)
}

type Client struct {
	agent  executor
	vmName string
}

// New creates a new Client
func New(agent *connect.AgentClient, vmName string) *Client {
	return &Client{agent: agent, vmName: vmName}
}

// QueryStatus checks the state of the VM
func (c *Client) QueryStatus(ctx context.Context) (*VMStatus, error) {
	raw, err := c.agent.Execute(ctx, c.vmName, cmdQueryStatus, nil)
	if err != nil {
		return nil, err
	}
	return connect.Parse[*VMStatus](raw)
}

// QueryMemory returns the memory of a VM
func (c *Client) QueryMemory(ctx context.Context) (*MemorySummary, error) {
	raw, err := c.agent.Execute(ctx, c.vmName, cmdQueryMemory, nil)
	if err != nil {
		return nil, err
	}
	return connect.Parse[*MemorySummary](raw)
}

// QueryCPUs lists the available CPUs for a vm
func (c *Client) QueryCPUs(ctx context.Context) ([]CPU, error) {
	raw, err := c.agent.Execute(ctx, c.vmName, cmdQueryCPUs, nil)
	if err != nil {
		return nil, err
	}
	return connect.Parse[[]CPU](raw)
}

// QueryBlockStats lists the blcok statistics of a VM
func (c *Client) QueryBlockStats(ctx context.Context) ([]BlockStat, error) {
	raw, err := c.agent.Execute(ctx, c.vmName, cmdQueryBlockStats, nil)
	if err != nil {
		return nil, err
	}
	return connect.Parse[[]BlockStat](raw)
}

// Reset restarts the VM
func (c *Client) Reset(ctx context.Context) error {
	_, err := c.agent.Execute(ctx, c.vmName, cmdReset, nil)
	return err
}

// PowerDown shuts down the VM
func (c *Client) PowerDown(ctx context.Context) error {
	_, err := c.agent.Execute(ctx, c.vmName, cmdPowerdown, nil)
	return err
}

// Stop force shut down the VM
func (c *Client) Stop(ctx context.Context) error {
	_, err := c.agent.Execute(ctx, c.vmName, cmdStop, nil)
	return err
}

// Resume starts the VM
func (c *Client) Resume(ctx context.Context) error {
	_, err := c.agent.Execute(ctx, c.vmName, cmdResume, nil)
	return err

}

// QueryCharDev lists the character devices attached to qwemu
func (c *Client) QueryCharDev(ctx context.Context) ([]CharDev, error) {
	raw, err := c.agent.Execute(ctx, c.vmName, cmdQueryChardev, nil)
	if err != nil {
		return nil, err
	}
	return connect.Parse[[]CharDev](raw)

}
