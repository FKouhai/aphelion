package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	vmUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aphelion_vm_up",
			Help: "Whether the VM is running (1) or not (0)",
		},
		[]string{"vm"},
	)
	vmMem = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aphelion_vm_memory_bytes",
			Help: "VM memory usage in bytes from cgroup memory.current",
		},
		[]string{"vm"},
	)
	vmCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aphelion_vm_cpu_usage_usec_total",
			Help: "Total CPU time consumed by the VM in microseconds",
		},
		[]string{"vm"},
	)
	vmCPUUser = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aphelion_vm_cpu_user_usec_total",
			Help: "User-space CPU time consumed by the VM in microseconds",
		},
		[]string{"vm"},
	)
	vmCPUSystem = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aphelion_vm_cpu_system_usec_total",
			Help: "Kernel-space CPU time consumed by the VM in microseconds",
		},
		[]string{"vm"},
	)
)

func registerMetrics() {
	prometheus.MustRegister(vmUp, vmMem, vmCPUUsage, vmCPUUser, vmCPUSystem)
}

func cgroupMemory(cgroupBase, vmName string) (uint64, error) {
	path := fmt.Sprintf("%s/microvm@%s.service/memory.current", cgroupBase, vmName)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading memory for %s: %w", vmName, err)
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

func cgroupCPU(cgroupBase, vmName string) (map[string]uint64, error) {
	path := fmt.Sprintf("%s/microvm@%s.service/cpu.stat", cgroupBase, vmName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cpu.stat for %s: %w", vmName, err)
	}
	stats := make(map[string]uint64)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			val, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				stats[fields[0]] = val
			}
		}
	}
	return stats, nil
}

func (m *VMManager) collect() {
	m.rediscover()

	type rawData struct {
		up  bool
		mem uint64
		cpu map[string]uint64
	}

	vmNames := m.knownVMs()
	gathered := make(map[string]rawData, len(vmNames))
	now := time.Now()

	for _, vmName := range vmNames {
		var d rawData

		raw, err := m.Execute(vmName, "query-status", nil)
		if err == nil {
			var status struct {
				Running bool `json:"running"`
			}
			if err := json.Unmarshal(raw, &status); err == nil {
				d.up = status.Running
			}
		}

		if mem, err := cgroupMemory(m.cgroupBase, vmName); err == nil {
			d.mem = mem
		}

		if cpu, err := cgroupCPU(m.cgroupBase, vmName); err == nil {
			d.cpu = cpu
		}

		gathered[vmName] = d
	}

	// Update Prometheus gauges.
	for vmName, d := range gathered {
		if d.up {
			vmUp.WithLabelValues(vmName).Set(1)
		} else {
			vmUp.WithLabelValues(vmName).Set(0)
		}
		vmMem.WithLabelValues(vmName).Set(float64(d.mem))
		if d.cpu != nil {
			vmCPUUsage.WithLabelValues(vmName).Set(float64(d.cpu["usage_usec"]))
			vmCPUUser.WithLabelValues(vmName).Set(float64(d.cpu["user_usec"]))
			vmCPUSystem.WithLabelValues(vmName).Set(float64(d.cpu["system_usec"]))
		}
	}

	// Compute CPU rate and store samples under lock.
	m.mu.Lock()
	elapsed := now.Sub(m.prevCPUTime).Seconds()
	for vmName, d := range gathered {
		sample := VMMetricsSample{Up: d.up, MemoryBytes: d.mem}
		if d.cpu != nil {
			usage := d.cpu["usage_usec"]
			if prev, ok := m.prevCPU[vmName]; ok && elapsed > 0 {
				delta := float64(int64(usage) - int64(prev))
				if delta >= 0 {
					sample.CPUPercent = delta / (elapsed * 1e6) * 100
				}
			}
			m.prevCPU[vmName] = usage
		}
		m.lastMetrics[vmName] = sample
	}
	m.prevCPUTime = now
	m.mu.Unlock()
}

func startMetrics(vms *VMManager, addr string, interval time.Duration) {
	registerMetrics()
	vms.collect() // initial sample so metrics are available immediately

	go func() {
		for {
			time.Sleep(interval)
			vms.collect()
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("metrics listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("metrics server: %v", err)
	}
}
