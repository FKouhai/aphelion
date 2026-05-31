package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
)

type VMManager struct {
	monitors map[string]*qmp.SocketMonitor
}

func discoverVMs(base string) (map[string]string, error) {
	sockets := make(map[string]string)
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".sock" {
			name := strings.TrimSuffix(d.Name(), ".sock")
			sockets[name] = path
		}
		return nil
	})
	return sockets, err
}

func NewVMManager(sockets map[string]string) (*VMManager, error) {
	monitors := make(map[string]*qmp.SocketMonitor)
	for name, path := range sockets {
		m, err := qmp.NewSocketMonitor("unix", path, 2*time.Second)
		if err != nil {
			return nil, fmt.Errorf("creating monitor for %s: %w", name, err)
		}
		if err := m.Connect(); err != nil {
			return nil, fmt.Errorf("connecting to %s: %w", name, err)
		}
		monitors[name] = m
	}
	return &VMManager{monitors: monitors}, nil
}

func (m *VMManager) Execute(vmName, method string, args any) (json.RawMessage, error) {
	monitor, ok := m.monitors[vmName]
	if !ok {
		return nil, fmt.Errorf("vm %q not found", vmName)
	}
	cmd, err := json.Marshal(struct {
		Execute   string `json:"execute"`
		Arguments any    `json:"arguments,omitempty"`
	}{Execute: method, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("marshaling command: %w", err)
	}
	return monitor.Run(cmd)
}

func (m *VMManager) Close() {
	for _, monitor := range m.monitors {
		monitor.Disconnect()
	}
}
