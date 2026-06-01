self:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.aphelion-agent;
in
{
  options.services.aphelion-agent = {
    enable = lib.mkEnableOption "Aphelion agent — QMP gateway and metrics exporter for microvm.nix VMs";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.system}.aphelion-agent;
      defaultText = lib.literalExpression "aphelion.packages.\${system}.aphelion-agent";
      description = "The aphelion-agent package to use.";
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 7373;
      description = "TCP port for the QMP gateway (used by the aphelion CLI to reach this host's VMs).";
    };

    metricsPort = lib.mkOption {
      type = lib.types.port;
      default = 9373;
      description = "HTTP port for the Prometheus metrics endpoint.";
    };

    vmBase = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/microvms";
      description = "Base directory where microvm.nix writes QMP sockets (one subdirectory per VM).";
    };

    cgroupBase = lib.mkOption {
      type = lib.types.path;
      default = "/sys/fs/cgroup/system.slice/system-microvm.slice";
      description = "Base path for microvm cgroup directories (used to resolve VM IP addresses and collect metrics).";
    };

    metricsInterval = lib.mkOption {
      type = lib.types.str;
      default = "15s";
      example = "1m";
      description = "How often to collect VM metrics (Go duration string, e.g. 15s, 1m).";
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = ''
        Open the agent port and metrics port in the firewall.
        The aphelion CLI tunnels over SSH so this is not required for normal use;
        enable it only if you want direct external access to the agent or metrics.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [
      cfg.port
      cfg.metricsPort
    ];

    systemd.services.aphelion-agent = {
      description = "Aphelion QMP gateway and metrics exporter";
      wantedBy = [ "multi-user.target" ];
      after = [
        "network.target"
        "microvm.target"
      ];
      wants = [ "microvm.target" ];

      serviceConfig = {
        ExecStart = lib.escapeShellArgs [
          (lib.getExe cfg.package)
          "--addr"
          "0.0.0.0:${toString cfg.port}"
          "--metrics-addr"
          "0.0.0.0:${toString cfg.metricsPort}"
          "--vm-base"
          (toString cfg.vmBase)
          "--cgroup-base"
          (toString cfg.cgroupBase)
          "--metrics-interval"
          cfg.metricsInterval
        ];

        User = "root";

        Restart = "on-failure";
        RestartSec = "5s";

        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        NoNewPrivileges = true;
        ReadOnlyPaths = [
          "/sys/fs/cgroup"
          "/proc"
          cfg.vmBase
        ];
      };
    };
  };
}
