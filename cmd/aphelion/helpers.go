package main

import (
	"context"
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

func discoverRemoteVMs(agent *connect.AgentClient, host config.Host) ([]string, error) {
	return agent.ListVMs(context.Background())
}
