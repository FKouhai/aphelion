self:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.aphelion-logd;
in
{
  options.services.aphelion-logd = {
    enable = lib.mkEnableOption "Aphelion log daemon — streams the systemd journal over WebSocket on request";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.system}.aphelion-logd;
      defaultText = lib.literalExpression "aphelion.packages.\${system}.aphelion-logd";
      description = "The aphelion-logd package to use.";
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 7374;
      description = "TCP port for the WebSocket log endpoint.";
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Open the logd port in the firewall.";
    };
  };

  config = lib.mkIf cfg.enable {
    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [ cfg.port ];

    users.users.aphelion-logd = {
      isSystemUser = true;
      group = "aphelion-logd";
      extraGroups = [ "systemd-journal" ];
    };
    users.groups.aphelion-logd = { };

    systemd.services.aphelion-logd = {
      description = "Aphelion journal log streaming daemon";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];

      serviceConfig = {
        ExecStart = lib.escapeShellArgs [
          (lib.getExe cfg.package)
          "--addr"
          "0.0.0.0:${toString cfg.port}"
        ];

        User = "aphelion-logd";
        Group = "aphelion-logd";

        Restart = "on-failure";
        RestartSec = "5s";

        NoNewPrivileges = true;
      };
    };
  };
}
