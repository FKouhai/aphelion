{ lib, buildGoModule, pname }:

buildGoModule {
  inherit pname;
  version = "0.1.0";
  src = ./.;
  subPackages = [ "cmd/${pname}" ];
  vendorHash = null;

  meta = {
    description = "TUI for managing microvm.nix VMs across NixOS hosts";
    mainProgram = pname;
  };
}
