self:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs.aphelion;

  hostToAttrs = h: lib.filterAttrs (_: v: v != null) {
    username = h.username;
    vm_username = if h.vmUsername != "" then h.vmUsername else null;
    address = h.address;
    display_name = h.displayName;
    port = h.port;
  };

  configFile = (pkgs.formats.yaml { }).generate "aphelion-config.yaml" {
    hosts = map hostToAttrs cfg.hosts;
  };

  wrappedPackage = pkgs.symlinkJoin {
    name = "aphelion";
    paths = [ cfg.package ];
    nativeBuildInputs = [ pkgs.makeWrapper ];
    postBuild = ''
      wrapProgram $out/bin/aphelion \
        --add-flags "--config ${configFile}"
    '';
  };
in
{
  options.programs.aphelion = {
    enable = lib.mkEnableOption "Aphelion — TUI for managing microvm.nix VMs across NixOS hosts";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.system}.aphelion;
      defaultText = lib.literalExpression "aphelion.packages.\${system}.aphelion";
      description = "The aphelion package to use.";
    };

    hosts = lib.mkOption {
      description = "List of NixOS hosts running aphelion-agent to manage.";
      default = [ ];
      type = lib.types.listOf (lib.types.submodule {
        options = {
          username = lib.mkOption {
            type = lib.types.str;
            description = "SSH username for the host.";
          };
          vmUsername = lib.mkOption {
            type = lib.types.str;
            default = "";
            description = "Default SSH username for VMs on this host. Falls back to username if unset.";
          };
          address = lib.mkOption {
            type = lib.types.str;
            description = "Hostname or IP address of the host.";
          };
          displayName = lib.mkOption {
            type = lib.types.str;
            description = "Human-readable name shown in the TUI and used as the identifier in CLI commands.";
          };
          port = lib.mkOption {
            type = lib.types.nullOr lib.types.port;
            default = null;
            description = "SSH port. Defaults to 22 if unset.";
          };
        };
      });
    };
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ wrappedPackage ];
  };
}
