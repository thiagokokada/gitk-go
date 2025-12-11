{
  lib,
  stdenv,
  fontconfig,
  freetype,
  libjpeg,
  libpng,
  makeWrapper,
  xorg,
  zlib,
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

  vendorHash = "sha256-irXhhMvUmMRo5x0tSFCn/zK+V90qBCGuCKAamRFNoDI=";

  nativeBuildInputs = lib.optionals stdenv.isLinux [
    makeWrapper
  ];

  postFixup = lib.optionalString stdenv.isLinux ''
    wrapProgram $out/bin/gitk-go \
    --set LD_LIBRARY_PATH ${
      with xorg;
      lib.makeLibraryPath [
        # XXX: not sure if all those libs are necessary
        libX11
        libXext
        libXft
        libXrender
        libXfixes
        libXcursor
        libXinerama
        libXrandr
        fontconfig
        freetype
        libpng
        libjpeg
        zlib
      ]
    }
  '';

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
