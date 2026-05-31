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
	"sync"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
)

// VMMetricsSample holds the latest collected metrics for one VM.
type VMMetricsSample struct {
	Up          bool    `json:"up"`
	MemoryBytes uint64  `json:"memory_bytes"`
	CPUPercent  float64 `json:"cpu_percent"`
}

type VMManager struct {
	vmBase  string
	sockMu  sync.RWMutex // protects sockets
	sockets map[string]string
	monMu   sync.RWMutex // protects monitors
	monitors map[string]*qmp.SocketMonitor
	cgroupBase  string
	mu          sync.RWMutex // protects lastMetrics / prevCPU
	lastMetrics map[string]VMMetricsSample
	prevCPU     map[string]uint64
	prevCPUTime time.Time
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

func NewVMManager(vmBase string, sockets map[string]string, cgroupBase string) *VMManager {
	m := &VMManager{
		vmBase:      vmBase,
		sockets:     sockets,
		monitors:    make(map[string]*qmp.SocketMonitor),
		cgroupBase:  cgroupBase,
		lastMetrics: make(map[string]VMMetricsSample),
		prevCPU:     make(map[string]uint64),
	}
	for name := range sockets {
		_ = m.connect(name) // best-effort; VMs may not be up yet
	}
	return m
}

// socketPath returns the path for a known VM, or ("", false) if unknown.
func (m *VMManager) socketPath(name string) (string, bool) {
	m.sockMu.RLock()
	defer m.sockMu.RUnlock()
	path, ok := m.sockets[name]
	return path, ok
}

// knownVMs returns a snapshot of the known VM names.
func (m *VMManager) knownVMs() []string {
	m.sockMu.RLock()
	defer m.sockMu.RUnlock()
	names := make([]string, 0, len(m.sockets))
	for name := range m.sockets {
		names = append(names, name)
	}
	return names
}

// connect (re)establishes the QMP socket connection for a single VM.
// Caller must not hold monMu or sockMu.
func (m *VMManager) connect(name string) error {
	path, ok := m.socketPath(name)
	if !ok {
		return fmt.Errorf("no socket path known for %s", name)
	}
	mon, err := qmp.NewSocketMonitor("unix", path, 2*time.Second)
	if err != nil {
		return err
	}
	if err := mon.Connect(); err != nil {
		return err
	}
	m.monMu.Lock()
	if old, exists := m.monitors[name]; exists {
		_ = old.Disconnect()
	}
	m.monitors[name] = mon
	m.monMu.Unlock()
	return nil
}

func (m *VMManager) Execute(vmName, method string, args any) (json.RawMessage, error) {
	cmd, err := json.Marshal(struct {
		Execute   string `json:"execute"`
		Arguments any    `json:"arguments,omitempty"`
	}{Execute: method, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("marshaling command: %w", err)
	}

	if _, known := m.socketPath(vmName); !known {
		return nil, fmt.Errorf("vm %q not found", vmName)
	}

	// Lazy-connect if the monitor doesn't exist yet (VM wasn't up at startup).
	m.monMu.RLock()
	_, connected := m.monitors[vmName]
	m.monMu.RUnlock()
	if !connected {
		if err := m.connect(vmName); err != nil {
			return nil, err
		}
	}

	// Attempt the command, reconnecting once if the monitor is stale.
	for attempt := range 2 {
		m.monMu.RLock()
		monitor := m.monitors[vmName]
		m.monMu.RUnlock()

		raw, runErr := monitor.Run(cmd)
		if runErr == nil {
			var wrapper struct {
				Return json.RawMessage `json:"return"`
			}
			if err := json.Unmarshal(raw, &wrapper); err != nil {
				return nil, fmt.Errorf("unwrapping QMP response: %w", err)
			}
			return wrapper.Return, nil
		}

		if attempt == 0 {
			_ = m.connect(vmName) // reconnect and retry once
		} else {
			return nil, runErr
		}
	}
	return nil, fmt.Errorf("vm %q: unreachable", vmName)
}

func (m *VMManager) List() []string {
	names := m.knownVMs()
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

// rediscover scans vmBase and registers any socket files not yet known.
func (m *VMManager) rediscover() {
	found, err := discoverVMs(m.vmBase)
	if err != nil {
		return
	}
	for name, path := range found {
		m.sockMu.RLock()
		_, known := m.sockets[name]
		m.sockMu.RUnlock()
		if known {
			continue
		}
		m.sockMu.Lock()
		m.sockets[name] = path
		m.sockMu.Unlock()
		_ = m.connect(name)
	}
}

func (m *VMManager) Metrics() map[string]VMMetricsSample {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]VMMetricsSample, len(m.lastMetrics))
	for k, v := range m.lastMetrics {
		out[k] = v
	}
	return out
}

func (m *VMManager) Close() {
	m.monMu.Lock()
	defer m.monMu.Unlock()
	for _, monitor := range m.monitors {
		_ = monitor.Disconnect()
	}
}
