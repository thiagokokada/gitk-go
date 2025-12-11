{
  lib,
  buildGoModule,
  version ? "unknown",
}:

buildGoModule {
  pname = "gitk-go";
  inherit version;

  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./cmd
      ./go.mod
      ./go.sum
      ./internal
      ./main.go
    ];
  };

  vendorHash = "sha256-AN8MRJw4EGM4KcE0B0sFOS+8K5AaPuFLVhCFjZJrVaQ=";

  ldflags = [
    "-s"
    "-w"
  ];

  meta = with lib; {
    description = "A lightweight Git history explorer written in Go";
    homepage = "https://github.com/thiagokokada/gitk-go";
    license = licenses.mit;
    mainProgram = "gitk-go";
  };
}
