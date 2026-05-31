package config

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

var (
	// ErrInvalidHostConfig parse error to avoid having invalid configs
	ErrInvalidHostConfig = errors.New("unable to parse host configuration")
)

/*
the yaml config would look something like:
hosts:
  - username: user1
    address: host1.example.com
    display_name: node1
    port: 22
  - username: user2
    vm_username: root
    address: host2.example.com
    display_name: node2
*/

// Host defines the shape of each item in the config file
type Host struct {
	Username    string `yaml:"username"`
	VMUsername  string `yaml:"vm_username,omitempty"`
	Address     string `yaml:"address"`
	DisplayName string `yaml:"display_name"`
	Port        *int   `yaml:"port,omitempty"`
}

// Config aggregates all the possible hosts
type Config struct {
	Hosts []Host `yaml:"hosts"`
}

// Validate ensures that the config has the required fields
func (c *Config) Validate() error {
	for _, h := range c.Hosts {
		switch {
		case h.Username == "":
			return fmt.Errorf("username is mandatory: %w", ErrInvalidHostConfig)
		case h.Address == "":
			return fmt.Errorf("address is mandatory: %w", ErrInvalidHostConfig)
		case h.DisplayName == "":
			return fmt.Errorf("display_name is mandatory: %w", ErrInvalidHostConfig)
		}
	}
	return nil
}

// Load reads the config file and instantiates the config
func Load(name string) (*Config, error) {
	var c Config
	f, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(f, &c); err != nil {
		return nil, err
	}
	if err = c.Validate(); err != nil {
		return nil, err
	}

	return &c, nil
}

// ByName lookup a host by its DisplayName
func (c *Config) ByName(name string) (*Host, error) {
	for _, v := range c.Hosts {
		if v.DisplayName == name {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("host %q not found %w", name, ErrInvalidHostConfig)
}
