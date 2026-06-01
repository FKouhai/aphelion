{
  lib,
  buildGoModule,
  installShellFiles,
  makeWrapper,
  pkg-config,
  systemd,
  pname,
  withCompletion ? false,
  withCGO ? false,
}:

buildGoModule (finalAttrs: {
  inherit pname;
  version = "0.5.0";
  src = ./.;
  subPackages = [ "cmd/${pname}" ];
  vendorHash = "sha256-COmkCWp17JUhSUnt81DqK8msswDYX0sx4iX7ssscgyg=";

  env.CGO_ENABLED = if withCGO then 1 else 0;
  ldflags = [
    "-s"
    "-w"
    "-X aphelion/pkg/version.Version=${finalAttrs.version}"
  ];

  nativeBuildInputs = lib.optional withCompletion installShellFiles
    ++ lib.optionals withCGO [ pkg-config makeWrapper ];
  buildInputs = lib.optional withCGO systemd;

  postInstall = lib.optionalString withCompletion ''
    installShellCompletion --cmd ${pname} \
      --bash <($out/bin/${pname} completion bash) \
      --fish <($out/bin/${pname} completion fish) \
      --zsh <($out/bin/${pname} completion zsh)
  '' + lib.optionalString withCGO ''
    wrapProgram $out/bin/${pname} \
      --prefix LD_LIBRARY_PATH : ${lib.makeLibraryPath [ systemd ]}
  '';

  meta = {
    description = "TUI for managing microvm.nix VMs across NixOS hosts";
    mainProgram = pname;
  };
})
