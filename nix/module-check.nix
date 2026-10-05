{
  lib,
  pkgs,
  module,
  package,
}:
let
  evaluate =
    settings:
    lib.evalModules {
      specialArgs = { inherit pkgs; };
      modules = [
        module
        {
          options.home.packages = lib.mkOption {
            type = lib.types.listOf lib.types.package;
            default = [ ];
          };
        }
        settings
      ];
    };
  disabled = evaluate { };
  enabled = evaluate { programs.pig-extensions.enable = true; };
  overridden = evaluate {
    programs.pig-extensions = {
      enable = true;
      package = pkgs.hello;
    };
  };
in
assert disabled.config.home.packages == [ ];
assert map toString enabled.config.home.packages == [ (toString package) ];
assert map toString overridden.config.home.packages == [ (toString pkgs.hello) ];
pkgs.runCommand "pig-extensions-home-module-check" { } ''
  touch "$out"
''
