{ pkgs }:
pkgs.mkShell {
  packages = with pkgs; [
    just
    k3d
    fzf
  ];
}
