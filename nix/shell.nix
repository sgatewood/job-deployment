{ pkgs }:
pkgs.mkShell {
  packages = with pkgs; [
    fzf
    go
    just
    k3d
  ];
}
