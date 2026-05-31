{
  pkgs ? import <nixpkgs> { },
}:
let

  build = pkgs.writeShellApplication {
    name = "build";
    text = ''
      cd "$(git rev-parse --show-toplevel)"
      nix build
      cd -
    '';
    meta.description = "builds the binary through nix";
  };

  lint = pkgs.writeShellApplication {
    name = "lint";
    text = ''
      cd "$(git rev-parse --show-toplevel)"
      golangci-lint run
      cd -
    '';
    meta.description = "lints the project";
  };

  run_tests = pkgs.writeShellApplication {
    name = "run_tests";
    text = ''
      cd "$(git rev-parse --show-toplevel)"
      go test -v ./...
      cd -
    '';
    meta.description = "runs the tests";
  };

  dev_txt = builtins.concatStringsSep "\n" (
    map (p: " ${p.name} -> ${p.meta.description}") customPkgs
  );

  dev_help = pkgs.writeShellApplication {
    name = "dev_help";
    text = ''
      echo " dev_help -> show this message"
      echo "${dev_txt}"
    '';
  };

  build_agent = pkgs.writeShellApplication {
    name = "build_agent";
    text = ''
      cd "$(git rev-parse --show-toplevel)"
      nix build .#aphelion-agent
      cd -
    '';
    meta.description = "builds the agent binary through nix";
  };

  customPkgs = [
    build
    build_agent
    lint
    run_tests
  ];

in

pkgs.mkShell {
  shellHook = "dev_help";
  packages =
    with pkgs;
    [
      go
      gopls
      gotools
      golangci-lint
    ]
    ++ customPkgs
    ++ [ dev_help ];
}
