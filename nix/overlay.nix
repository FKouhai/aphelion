final: prev: {
  aphelion = prev.callPackage ../default.nix {
    pname = "aphelion";
    withCompletion = true;
  };
  aphelion-agent = prev.callPackage ../default.nix {
    pname = "aphelion-agent";
  };
  aphelion-logd = prev.callPackage ../default.nix {
    pname = "aphelion-logd";
    withCGO = true;
  };
}
