{ pkgs, self, ... }:
let
  # The release workflow writes nix/version into the commit that the latest tag names.
  version =
    if builtins.pathExists ./version then
      pkgs.lib.trim (builtins.readFile ./version)
    else
      "0.0.0-${self.sourceInfo.shortRev or (self.sourceInfo.dirtyShortRev or "dirty")}";
in
pkgs.buildGo126Module {
  pname = "wonkey";
  inherit version;
  src = self;
  vendorHash = "sha256-4RUfprf1lP82akubVGSLvobWXTQBQvMtc0+fHlHAsuA=";
  subPackages = [ "cmd/wonkey" ];
  env.CGO_ENABLED = 0;
  ldflags = [
    "-s"
    "-w"
  ];
  meta = {
    mainProgram = "wonkey";
    platforms = pkgs.lib.platforms.linux;
  };
}
