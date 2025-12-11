{
  description = "gitk-go";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      forAllSystems = nixpkgs.lib.genAttrs [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
      version = "nix-${self.shortRev or self.dirtyShortRev or "unknown-dirty"}";
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = self.packages.${system}.gitk-go;
          gitk-go = pkgs.callPackage ./gitk-go.nix { inherit version; };
          gitk-go-purego = self.packages.${system}.gitk-go.override { withGitCli = false; };
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            inputsFrom = [ self.packages.${system}.default ];
          };
        }
      );
    };
}
