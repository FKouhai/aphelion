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

func collect(vms *VMManager, cgroupBase string) {
	for vmName := range vms.monitors {
		raw, err := vms.Execute(vmName, "query-status", nil)
		if err != nil {
			vmUp.WithLabelValues(vmName).Set(0)
			continue
		}
		var status struct {
			Running bool `json:"running"`
		}
		if err := json.Unmarshal(raw, &status); err != nil || !status.Running {
			vmUp.WithLabelValues(vmName).Set(0)
		} else {
			vmUp.WithLabelValues(vmName).Set(1)
		}

		if mem, err := cgroupMemory(cgroupBase, vmName); err == nil {
			vmMem.WithLabelValues(vmName).Set(float64(mem))
		}

		if cpu, err := cgroupCPU(cgroupBase, vmName); err == nil {
			if v, ok := cpu["usage_usec"]; ok {
				vmCPUUsage.WithLabelValues(vmName).Set(float64(v))
			}
			if v, ok := cpu["user_usec"]; ok {
				vmCPUUser.WithLabelValues(vmName).Set(float64(v))
			}
			if v, ok := cpu["system_usec"]; ok {
				vmCPUSystem.WithLabelValues(vmName).Set(float64(v))
			}
		}
	}
}

func startMetrics(vms *VMManager, addr, cgroupBase string, interval time.Duration) {
	registerMetrics()

	go func() {
		for {
			collect(vms, cgroupBase)
			time.Sleep(interval)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("metrics listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("metrics server: %v", err)
	}
}
