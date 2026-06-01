{
  description = "Aphelion — TUI for managing microvm.nix VMs across NixOS hosts";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: {
        aphelion = pkgs.callPackage ./default.nix {
          pname = "aphelion";
          withCompletion = true;
        };
        aphelion-logd = pkgs.callPackage ./default.nix { pname = "aphelion-logd"; withCGO = true; };
        aphelion-agent = pkgs.callPackage ./default.nix { pname = "aphelion-agent"; };
        default = pkgs.callPackage ./default.nix {
          pname = "aphelion";
          withCompletion = true;
        };
      });

      overlays.default = import ./nix/overlay.nix;

      nixosModules = {
        aphelion = import ./nix/modules/aphelion self;
        aphelion-agent = import ./nix/modules/aphelion-agent self;
        aphelion-logd = import ./nix/modules/aphelion-logd self;
        default = import ./nix/modules/aphelion-agent self;
      };

      devShells = forAllSystems (pkgs: {
        default = import ./shell.nix { inherit pkgs; };
      });
    };
}
