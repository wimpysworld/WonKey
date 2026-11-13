{
  description = "Nix flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-26.05-darwin";
    tailor.url = "github:wimpysworld/tailor/latest";
  };

  outputs =
    {
      self,
      nixpkgs,
      tailor,
      ...
    }:
    let
      supportedSystems = [
        "x86_64-darwin"
        "x86_64-linux"
        "aarch64-darwin"
        "aarch64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          package = import ./nix/package.nix { inherit pkgs self; };
        in
        # Expose the project package under its pname and as the default package.
        pkgs.lib.optionalAttrs (builtins.pathExists ./nix/package.nix) {
          ${package.pname} = package;
          default = package;
        }
        // pkgs.lib.optionalAttrs (builtins.pathExists ./nix/lib/blender.nix) {
          blender-agent-bridge-extension = (import ./nix/lib/blender.nix { inherit pkgs; }).extension;
        }
      );
      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          # Keep the development shell available when Tailor has no package for this system.
          tailorPkgs = tailor.packages.${system} or { };
          browsers = pkgs.playwright-driver.browsers-chromium;
          playwright-test = pkgs.playwright-test.overrideAttrs (old: {
            installPhase = builtins.replaceStrings
              [ "${pkgs.playwright-driver.browsers}" ]
              [ "${browsers}" ]
              old.installPhase;
          });
          playwright-mcp-package = pkgs.playwright-mcp.override {
            inherit playwright-test;
            playwright-driver = pkgs.playwright-driver // { inherit browsers; };
          };
          playwright-mcp = pkgs.writeShellApplication {
            name = "playwright-mcp";
            text = ''
              if [[ -n "''${HTTPS_PROXY:-}" ]]; then
                exec ${playwright-mcp-package}/bin/playwright-mcp --proxy-server "$HTTPS_PROXY" "$@"
              fi
              exec ${playwright-mcp-package}/bin/playwright-mcp "$@"
            '';
          };
        in
        {
          default = pkgs.mkShell {
            shellHook = import ./nix/hooks.nix { inherit pkgs; };
            packages =
              with pkgs;
              [
                actionlint
                gh
                just
                playwright-mcp
              ]
              ++ (if tailorPkgs ? tailor then [ tailorPkgs.tailor ] else [ ])
              ++ import ./nix/loader.nix { inherit pkgs; };
          };
        }
        //
          pkgs.lib.optionalAttrs
            (system == "x86_64-linux" && builtins.pathExists ./nix/shells/go-ebitengine-android.nix)
            {
              android = import ./nix/shells/go-ebitengine-android.nix { inherit pkgs; };
            }
      );
    };
}
