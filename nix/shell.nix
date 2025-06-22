{ pkgs }:
pkgs.mkShell {
  packages = with pkgs; [
    fzf
    go
    golangci-lint
    just
    k3d
    kubebuilder
  ];
}
