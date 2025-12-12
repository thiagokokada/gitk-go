{
  lib,
  stdenv,
  buildGoModule,
  fontconfig,
  freetype,
  git,
  libjpeg,
  libpng,
  makeWrapper,
  xorg,
  zlib,
  version ? "unknown",
  withGitCli ? true,
  withSyntaxHighlight ? true,
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

  vendorHash = "sha256-KDae1LnFARj6Pz+Gsup8bQgyc6cp6VQq9uxVwUCTqBY=";

  nativeBuildInputs = [
    makeWrapper
  ];

  env.GOEXPERIMENT = "greenteagc";

  postFixup = (
    # bash
    lib.concatStringsSep " " [
      "wrapProgram $out/bin/gitk-go"
      (lib.optionalString stdenv.isLinux "--set LD_LIBRARY_PATH ${
        lib.makeLibraryPath [
          # XXX: not sure if all those libs are necessary
          xorg.libX11
          xorg.libXext
          xorg.libXft
          xorg.libXrender
          xorg.libXfixes
          xorg.libXcursor
          xorg.libXinerama
          xorg.libXrandr
          fontconfig
          freetype
          libpng
          libjpeg
          zlib
        ]
      }")
      (lib.optionalString withGitCli "--prefix PATH : ${lib.getExe git}")
    ]
  );

  tags =
    lib.optionals withGitCli [ "gitcli" ]
    ++ lib.optionals (!withSyntaxHighlight) [ "nosyntaxhighlight" ];

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
