{
  lib,
  stdenv,
  buildGoModule,
  fontconfig,
  freetype,
  git,
  libjpeg,
  libpng,
  makeBinaryWrapper,
  xorg,
  zlib,
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

  vendorHash = "sha256-ErILkTre+PVksMcIiAJrQimm8YzUnffl+R522ZT+rLE=";

  nativeBuildInputs = [
    git
    makeBinaryWrapper
  ];

  env.GOEXPERIMENT = "greenteagc";

  postFixup =
    let
      linuxLibs = lib.makeLibraryPath [
        xorg.libX11
        xorg.libXext
        xorg.libXrender
        xorg.libXft
        xorg.libXfixes
        xorg.libXcursor
        xorg.libXinerama
        xorg.libXrandr
        xorg.libXScrnSaver
        fontconfig
        freetype
        libpng
        libjpeg
        zlib
      ];

      gitPath = "--prefix PATH : ${lib.makeBinPath [ git ]}";
    in
    ''
      mv $out/bin/gitk-go $out/bin/.gitk-go-unwrapped
    ''
    +
      # XXX: purego (used by modernc.org/tk9.0) is kinda cursed, it makes the Go
      # binaries to be linked to the dynamic linker even when CGO is disabled.
      # The issue is that the libraries that are shipped with tk9.0 also get
      # linked with the dynamic linker used by Go, but in non-NixOS systems
      # Go will get confused trying to load the libs from Nix with the system
      # dynamic loader (and this doesn't work).
      # Ideally we would patch purego/tk9.0 to allow overriding this (e.g., by
      # allowing loading tk9.0 from nixpkgs instead of using the one bundled in
      # the package), but for now forcing the binary to load with Nix's dynamic
      # linker, even if this is an unholy hack.
      lib.optionalString stdenv.isLinux ''
        makeWrapper ${stdenv.cc.bintools.dynamicLinker} \
          $out/bin/gitk-go \
          --add-flags $out/bin/.gitk-go-unwrapped \
          --argv0 gitk-go \
          --set LD_LIBRARY_PATH ${linuxLibs} \
          ${gitPath}
      ''
    + lib.optionalString stdenv.isDarwin ''
      makeWrapper $out/bin/.gitk-go-unwrapped \
        $out/bin/gitk-go \
        ${gitPath}
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
