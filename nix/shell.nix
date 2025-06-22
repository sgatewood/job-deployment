{ pkgs }:
pkgs.mkShell {
  packages = with pkgs; [
    fzf
    go
    golangci-lint
    gotools
    just
    k3d
    kubebuilder
  ];
}
