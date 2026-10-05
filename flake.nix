{
  description = "PiG with the six pig-extensions factories fused in-process";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
      package = pkgs.callPackage ./nix/package.nix { };
    in
    {
      packages.${system} = {
        default = package;
        pig-extensions = package;
      };
      apps.${system}.default = {
        type = "app";
        program = "${package}/bin/pig-extensions";
        meta.description = "Run PiG with the six fused extensions";
      };
      checks.${system} = {
        build = package;
        home-module = import ./nix/module-check.nix {
          inherit pkgs package;
          inherit (pkgs) lib;
          module = self.homeManagerModules.default;
        };
      };
      homeManagerModules.default =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        let
          cfg = config.programs.pig-extensions;
        in
        {
          options.programs.pig-extensions = {
            enable = lib.mkEnableOption "PiG with fused pig-extensions";
            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
              description = "The fused Piglet Binary package.";
            };
          };
          config = lib.mkIf cfg.enable {
            home.packages = [ cfg.package ];
          };
        };
    };
}
