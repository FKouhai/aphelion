{
  lib,
  buildGoModule,
  installShellFiles,
  pname,
  withCompletion ? false,
}:

buildGoModule {
  inherit pname;
  version = "0.1.0";
  src = ./.;
  subPackages = [ "cmd/${pname}" ];
  vendorHash = "sha256-kfCrhdzjob4d+pJdnmsUpMBG8eVSrjqXgN7epr/obME=";

  nativeBuildInputs = lib.optional withCompletion installShellFiles;

  postInstall = lib.optionalString withCompletion ''
    installShellCompletion --cmd ${pname} \
      --bash <($out/bin/${pname} completion bash) \
      --fish <($out/bin/${pname} completion fish) \
      --zsh <($out/bin/${pname} completion zsh)
  '';

  meta = {
    description = "TUI for managing microvm.nix VMs across NixOS hosts";
    mainProgram = pname;
  };
}
