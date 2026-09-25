{
  description = "Nix flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-26.05-darwin";
    nix-packages.url = "github:wimpysworld/nix-packages";
    nix-packages.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    {
      nixpkgs,
      nix-packages,
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
      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          tailorPkgs = nix-packages.packages.${system} or { };
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
              ++ import ./nix/loader.nix { inherit pkgs; }
              ++ (if tailorPkgs ? tailor then [ tailorPkgs.tailor ] else [ ]);
          };
        }
      );
    };
}
