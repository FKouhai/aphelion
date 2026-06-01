# Aphelion

CLI for managing [microvm.nix](https://github.com/astro/microvm.nix) VMs across NixOS hosts. Connect to VMs by name, inspect their state, control their lifecycle, and stream logs — all tunnelled over an existing SSH connection to the host, with no extra ports exposed.

## Architecture

```
your machine
  └─ aphelion CLI
       │  SSH to host (port 22)
       └──────────────────────────────► NixOS host (copernico)
                                          └─ aphelion-agent  :7373  ← TCP-forwarded over SSH
                                               │  QMP socket per VM
                                               ├──► microvm@worker01
                                               │      └─ aphelion-logd  :7374  ← direct to VM IP
                                               ├──► microvm@worker02
                                               │      └─ aphelion-logd  :7374
                                               └──► ...
```

`aphelion-agent` runs on each NixOS host that manages VMs. It:

- Discovers VMs by scanning QMP sockets under `/var/lib/microvms/`
- Forwards QMP commands (start, stop, query-status, …) from the CLI to the VM
- Resolves VM IP addresses by walking cgroup → QEMU PID → `/proc/<pid>/cmdline` → MAC → ARP table
- Exports Prometheus metrics on port 9373

`aphelion-logd` runs inside each VM. It streams the systemd journal over a WebSocket connection on port 7374 when requested by the CLI.

The CLI connects to the host over SSH, then port-forwards to the agent over that same connection. No firewall rules are required on the host for normal use.

## Demo

[![asciicast](https://asciinema.org/a/DyvoK3FMWPMjKBII.svg)](https://asciinema.org/a/DyvoK3FMWPMjKBII)


## Installation

### Nix flake

Add aphelion to your flake inputs:

```nix
inputs.aphelion.url = "github:FKouhai/aphelion";
```

#### nixpkgs overlay

To expose `pkgs.aphelion`, `pkgs.aphelion-agent`, and `pkgs.aphelion-logd` in your own nixpkgs instance:

```nix
nixpkgs.overlays = [ aphelion.overlays.default ];
```

Or run the CLI directly without installing:

```
nix run github:FKouhai/aphelion
```

---

### CLI (`aphelion`) — NixOS module

Import `nixosModules.aphelion` on the machine where you run the CLI. This installs the binary and generates `~/.config/aphelion/config.yaml` from your Nix config:

```nix
{
  imports = [ aphelion.nixosModules.aphelion ];

  programs.aphelion = {
    enable = true;
    hosts = [
      {
        displayName = "copernico";
        address = "192.168.0.19";
        username = "nixos";       # SSH user on the host
        vmUsername = "nixos";     # SSH user inside VMs (used by attach)
        # port = 22;              # host SSH port, defaults to 22
      }
    ];
  };
}
```

`displayName` is what you pass to every command as `<host>`.

#### All module options — `programs.aphelion`

| Option | Default | Description |
|---|---|---|
| `enable` | `false` | Install aphelion and generate the config file |
| `package` | flake package | `aphelion` package to use |
| `hosts` | `[]` | List of hosts to manage (see submodule options below) |

Each entry in `hosts` accepts:

| Option | Default | Description |
|---|---|---|
| `username` | — | SSH username for the host |
| `vmUsername` | `""` | Default SSH username for VMs; falls back to `username` |
| `address` | — | Hostname or IP address |
| `displayName` | — | Identifier used in CLI commands and the TUI |
| `port` | `null` | SSH port; defaults to 22 if unset |

---

### Agent (`aphelion-agent`) — NixOS module

Import `nixosModules.aphelion-agent` on each NixOS host that runs microvms:

```nix
{
  imports = [ aphelion.nixosModules.aphelion-agent ];

  services.aphelion-agent.enable = true;
}
```

The agent listens on `0.0.0.0:7373` by default. The CLI tunnels over SSH so this port does not need to be open in the firewall. To allow direct external access (e.g. for Prometheus scraping):

```nix
services.aphelion-agent = {
  enable = true;
  openFirewall = true;   # opens ports 7373 and 9373
};
```

#### All module options — `services.aphelion-agent`

| Option | Default | Description |
|---|---|---|
| `enable` | `false` | Enable the agent service |
| `package` | flake package | `aphelion-agent` package to use |
| `port` | `7373` | TCP port for the QMP gateway |
| `metricsPort` | `9373` | HTTP port for Prometheus metrics |
| `vmBase` | `/var/lib/microvms` | Directory containing microvm QMP sockets |
| `cgroupBase` | `/sys/fs/cgroup/system.slice/system-microvm.slice` | Base path for microvm cgroup directories |
| `metricsInterval` | `15s` | Metrics collection interval (Go duration string) |
| `openFirewall` | `false` | Open `port` and `metricsPort` in the firewall |

---

### Log daemon (`aphelion-logd`) — NixOS module

Import `nixosModules.aphelion-logd` inside each VM's NixOS configuration to enable log streaming:

```nix
{
  imports = [ aphelion.nixosModules.aphelion-logd ];

  services.aphelion-logd.enable = true;
}
```

The daemon runs as a dynamic user with access to the systemd journal and listens on `0.0.0.0:7374`. The CLI reaches it directly over the VM's bridge network IP (resolved via the agent).

#### All module options — `services.aphelion-logd`

| Option | Default | Description |
|---|---|---|
| `enable` | `false` | Enable the log daemon service |
| `package` | flake package | `aphelion-logd` package to use |
| `port` | `7374` | TCP port for the WebSocket log endpoint |
| `openFirewall` | `false` | Open `port` in the firewall |

---

## Usage

### Status

List all VMs across all configured hosts and their current state:

```
aphelion status
```

### Attach

Open an interactive SSH session to a VM by name:

```
aphelion attach <host> <vm>
aphelion attach copernico worker01
```

The VM is looked up by name through the agent; no need to know its IP. The session runs over the existing host SSH connection.

Options:
- `--user <name>` — override the SSH username (defaults to `vmUsername` from config)
- `--port <port>` — override the VM SSH port (default `22`)

### Logs

Stream the systemd journal from a VM in real time (requires `aphelion-logd` running inside the VM):

```
aphelion logs <host> <vm>
aphelion logs copernico worker01
```

Each line is printed as `Jan 02 15:04:05 <unit>: <message>`. Press `Ctrl-C` to stop.

### VM lifecycle

```
aphelion vm stop    <host> <vm>
aphelion vm restart <host> <vm>
aphelion vm resume  <host> <vm>
```

### Global flags

```
--config <path>      config file (default ~/.config/aphelion/config.yaml)
--agent-port <port>  agent port to forward to (default 7373)
--logd-port <port>   logd port to connect to (default 7374)
```

## Building from source

```
go build ./cmd/aphelion/
go build ./cmd/aphelion-agent/
CGO_ENABLED=1 go build ./cmd/aphelion-logd/   # requires libsystemd-dev
```

Tests:

```
go test ./...
```
