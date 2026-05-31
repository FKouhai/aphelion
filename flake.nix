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
        aphelion-agent = pkgs.callPackage ./default.nix { pname = "aphelion-agent"; };
        default = pkgs.callPackage ./default.nix {
          pname = "aphelion";
          withCompletion = true;
        };
      });

      overlays.default = import ./nix/overlay.nix;

      nixosModules.default = import ./nix/module.nix self;

      devShells = forAllSystems (pkgs: {
        default = import ./shell.nix { inherit pkgs; };
      });
    };
}
