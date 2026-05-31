package config

import (
	"errors"
	"os"
	"testing"
)

func validHost() Host {
	return Host{
		Username:    "stub_user",
		Address:     "192.168.1.10",
		DisplayName: "stub_machine",
		Port:        new(int),
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "empty hosts",
			config:  Config{},
			wantErr: false,
		},
		{
			name:    "valid host",
			config:  Config{Hosts: []Host{validHost()}},
			wantErr: false,
		},
		{
			name:    "valid host with vm_username",
			config:  Config{Hosts: []Host{{Username: "stub_user", VMUsername: "root", Address: "192.168.1.10", DisplayName: "stub_machine"}}},
			wantErr: false,
		},
		{
			name:    "missing username",
			config:  Config{Hosts: []Host{{Address: "192.168.1.10", DisplayName: "stub_machine"}}},
			wantErr: true,
		},
		{
			name:    "missing address",
			config:  Config{Hosts: []Host{{Username: "stub_user", DisplayName: "stub_machine"}}},
			wantErr: true,
		},
		{
			name:    "missing display_name",
			config:  Config{Hosts: []Host{{Username: "stub_user", Address: "192.168.1.10"}}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidHostConfig) {
				t.Errorf("expected ErrInvalidHostConfig, got %v", err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	validYAML := `
hosts:
  - username: stub_user
    address: 192.168.1.10
    display_name: stub_machine
    port: 22
`
	validYAMLWithVMUsername := `
hosts:
  - username: stub_user
    vm_username: root
    address: 192.168.1.10
    display_name: stub_machine
`
	invalidYAML := `hosts: [`

	missingFieldYAML := `
hosts:
  - username: stub_user
    address: 192.168.1.10
`

	writeTemp := func(t *testing.T, content string) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
		return f.Name()
	}

	t.Run("valid config", func(t *testing.T) {
		cfg, err := Load(writeTemp(t, validYAML))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Hosts) != 1 {
			t.Errorf("expected 1 host, got %d", len(cfg.Hosts))
		}
		if cfg.Hosts[0].DisplayName != "stub_machine" {
			t.Errorf("unexpected display_name: %s", cfg.Hosts[0].DisplayName)
		}
	})

	t.Run("valid config with vm_username", func(t *testing.T) {
		cfg, err := Load(writeTemp(t, validYAMLWithVMUsername))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Hosts[0].VMUsername != "root" {
			t.Errorf("unexpected vm_username: %s", cfg.Hosts[0].VMUsername)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := Load("/nonexistent/config.yaml")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		_, err := Load(writeTemp(t, invalidYAML))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		_, err := Load(writeTemp(t, missingFieldYAML))
		if !errors.Is(err, ErrInvalidHostConfig) {
			t.Errorf("expected ErrInvalidHostConfig, got %v", err)
		}
	})
}

func TestByName(t *testing.T) {
	cfg := Config{
		Hosts: []Host{
			validHost(),
			{Username: "stub_user", Address: "192.168.1.11", DisplayName: "rpi-node"},
		},
	}

	t.Run("found", func(t *testing.T) {
		h, err := cfg.ByName("stub_machine")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h.Address != "192.168.1.10" {
			t.Errorf("unexpected address: %s", h.Address)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := cfg.ByName("unknown")
		if !errors.Is(err, ErrInvalidHostConfig) {
			t.Errorf("expected ErrInvalidHostConfig, got %v", err)
		}
	})
}
