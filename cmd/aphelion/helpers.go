package main

import (
	"fmt"
	"os"
	"path/filepath"

	"aphelion/pkg/config"
	"aphelion/pkg/connect"
)

func loadConfig() (*config.Config, error) {
	path := cfgFile
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("finding home dir: %w", err)
		}
		path = filepath.Join(home, ".config", "aphelion", "config.yaml")
	}
	return config.Load(path)
}

// discoverRemoteVMs asks the agent which VMs it knows about by attempting
// query-status for each VM name returned by the agent's socket discovery.
// For now this is a placeholder — the agent will expose a list endpoint later.
func discoverRemoteVMs(agent *connect.AgentClient, host config.Host) ([]string, error) {
	// TODO: replace with a dedicated agent "list-vms" command once the
	// agent wire protocol is extended to support it.
	return nil, fmt.Errorf("discoverRemoteVMs not yet implemented")
}
