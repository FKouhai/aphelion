package main

import (
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aphelion-agent",
	Short: "QMP gateway and metrics exporter for microvm.nix VMs",
	RunE:  run,
}

var (
	vmBase          string
	agentAddr       string
	metricsAddr     string
	cgroupBase      string
	metricsInterval time.Duration
)

func init() {
	rootCmd.Flags().StringVar(&vmBase, "vm-base", "/var/lib/microvms", "base directory for microvm sockets")
	rootCmd.Flags().StringVar(&agentAddr, "addr", "0.0.0.0:7373", "TCP address for the QMP gateway")
	rootCmd.Flags().StringVar(&metricsAddr, "metrics-addr", "0.0.0.0:9373", "HTTP address for the metrics endpoint")
	rootCmd.Flags().StringVar(&cgroupBase, "cgroup-base", "/sys/fs/cgroup/system.slice/system-microvm.slice", "base path for microvm cgroup files")
	rootCmd.Flags().DurationVar(&metricsInterval, "metrics-interval", 15*time.Second, "metrics collection interval")
}

func run(cmd *cobra.Command, args []string) error {
	sockets, err := discoverVMs(vmBase)
	if err != nil {
		log.Printf("warning: initial vm discovery: %v", err)
		sockets = make(map[string]string)
	}
	log.Printf("discovered %d VMs at startup", len(sockets))

	vms := NewVMManager(vmBase, sockets, cgroupBase)
	defer vms.Close()

	go startMetrics(vms, metricsAddr, metricsInterval)

	server := NewServer(agentAddr, vms)
	return server.Listen()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
