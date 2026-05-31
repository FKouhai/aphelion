package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
)

type VMManager struct {
	monitors   map[string]*qmp.SocketMonitor
	cgroupBase string
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

func NewVMManager(sockets map[string]string, cgroupBase string) (*VMManager, error) {
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
	return &VMManager{monitors: monitors, cgroupBase: cgroupBase}, nil
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
	raw, err := monitor.Run(cmd)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Return json.RawMessage `json:"return"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("unwrapping QMP response: %w", err)
	}
	return wrapper.Return, nil
}

func (m *VMManager) List() []string {
	names := make([]string, 0, len(m.monitors))
	for name := range m.monitors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var macRe = regexp.MustCompile(`mac=([0-9a-fA-F]{2}(?::[0-9a-fA-F]{2}){5})`)

func (m *VMManager) GetAddr(vmName string) (string, error) {
	procsPath := filepath.Join(m.cgroupBase, "microvm@"+vmName+".service", "cgroup.procs")
	data, err := os.ReadFile(procsPath)
	if err != nil {
		return "", fmt.Errorf("reading cgroup procs for %s: %w", vmName, err)
	}
	pids := strings.Fields(strings.TrimSpace(string(data)))
	if len(pids) == 0 {
		return "", fmt.Errorf("no PID found for %s", vmName)
	}

	cmdline, err := os.ReadFile("/proc/" + pids[0] + "/cmdline")
	if err != nil {
		return "", fmt.Errorf("reading cmdline for PID %s: %w", pids[0], err)
	}
	args := strings.ReplaceAll(string(cmdline), "\x00", " ")
	match := macRe.FindStringSubmatch(args)
	if match == nil {
		return "", fmt.Errorf("no MAC found in QEMU cmdline for %s", vmName)
	}
	mac := strings.ToLower(match[1])

	arp, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return "", fmt.Errorf("reading ARP table: %w", err)
	}
	for _, line := range strings.Split(string(arp), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 4 && strings.ToLower(fields[3]) == mac {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("MAC %s not in ARP table (try pinging the VM first)", mac)
}

func (m *VMManager) Close() {
	for _, monitor := range m.monitors {
		monitor.Disconnect()
	}
}
