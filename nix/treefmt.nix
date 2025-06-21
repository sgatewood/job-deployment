{ pkgs, ... }:
{
  projectRootFile = "flake.nix";
  programs = builtins.mapAttrs (k: v: v // { enable = true; }) {
    nixfmt = { };
    just = { };
    shfmt = {
      includes = [
        ".envrc"
      ];
    };
  };

  settings.global.excludes = [
    ".editorconfig"
    "justfile"
  ];
}
