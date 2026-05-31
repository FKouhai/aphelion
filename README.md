# Aphelion

CLI for managing [microvm.nix](https://github.com/astro/microvm.nix) VMs across NixOS hosts. Connect to VMs by name, inspect their state, and control their lifecycle — all tunnelled over an existing SSH connection to the host, with no extra ports exposed.

## Architecture

```
your machine
  └─ aphelion CLI
       │  SSH to host (port 22)
       └──────────────────────────────► NixOS host (copernico)
                                          └─ aphelion-agent  :7373  ← TCP-forwarded over SSH
                                               │  QMP socket per VM
                                               ├──► microvm@worker01  (libvirt/firecracker)
                                               ├──► microvm@worker02
                                               └──► ...
```

`aphelion-agent` runs on each NixOS host that manages VMs. It:

- Discovers VMs by scanning QMP sockets under `/var/lib/microvms/`
- Forwards QMP commands (start, stop, query-status, …) from the CLI to the VM
- Resolves VM IP addresses by walking cgroup → QEMU PID → `/proc/<pid>/cmdline` → MAC → ARP table
- Exports Prometheus metrics on port 9373

The CLI connects to the host over SSH, then port-forwards to the agent over that same SSH connection. No firewall rules are required on the host for normal use.

## Installation

### Nix flake

Add aphelion to your flake inputs:

```nix
inputs.aphelion.url = "github:FKouhai/aphelion";
```

#### CLI (`aphelion`)

Install into your user profile or home-manager config:

```nix
environment.systemPackages = [ aphelion.packages.${system}.aphelion ];
# or
home.packages = [ aphelion.packages.${system}.aphelion ];
```

Or run it directly without installing:

```
nix run github:FKouhai/aphelion
```

#### nixpkgs overlay

To expose `pkgs.aphelion` and `pkgs.aphelion-agent` in your own nixpkgs instance:

```nix
nixpkgs.overlays = [ aphelion.overlays.default ];
```

### Agent (`aphelion-agent`) — NixOS module

Import the NixOS module on each host that runs microvms:

```nix
{
  imports = [ aphelion.nixosModules.default ];

  services.aphelion-agent.enable = true;
}
```

The agent listens on `0.0.0.0:7373` by default. Because the CLI tunnels over SSH this port does not need to be open in the firewall. To allow direct external access (e.g. for Prometheus scraping from another host):

```nix
services.aphelion-agent = {
  enable = true;
  openFirewall = true;   # opens ports 7373 and 9373
};
```

#### All module options

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

## Configuration

The CLI reads `~/.config/aphelion/config.yaml` by default (`--config` to override).

```yaml
hosts:
  - display_name: copernico
    address: 192.168.0.19
    username: nixos        # SSH user on the host
    vm_username: nixos     # SSH user inside VMs (used by attach)
    # port: 22             # host SSH port, defaults to 22
```

`display_name` is what you pass to every command as `<host>`.

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
```

```
aphelion attach copernico worker01
```

The VM is looked up by name through the agent; no need to know its IP. The session runs over the existing host SSH connection.

Options:
- `--user <name>` — override the SSH username (defaults to `vm_username` from config)
- `--port <port>` — override the VM SSH port (default `22`)

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
```

## Building from source

```
go build ./cmd/aphelion/
go build ./cmd/aphelion-agent/
```

Tests:

```
go test ./...
```
