{
  lib,
  stdenv,
  buildGoModule,
  fontconfig,
  freetype,
  git,
  libX11,
  libXScrnSaver,
  libXcursor,
  libXext,
  libXfixes,
  libXft,
  libXinerama,
  libXrandr,
  libXrender,
  libjpeg,
  libpng,
  makeBinaryWrapper,
  patchelf,
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

  vendorHash = "sha256-n7kmCfrBoCxKylUNWOUYl9x9ptClpzvpdTpeKeOEtFM=";

  nativeBuildInputs = [
    git
    makeBinaryWrapper
    patchelf
  ];

  postFixup =
    let
      linuxLibs = lib.makeLibraryPath [
        libX11
        libXext
        libXrender
        libXft
        libXfixes
        libXcursor
        libXinerama
        libXrandr
        libXScrnSaver
        fontconfig
        freetype
        libpng
        libjpeg
        zlib
      ];

      gitPath = "--prefix PATH : ${lib.makeBinPath [ git ]}";
      wrappedBinary = "$out/bin/.gitk-go-unwrapped";
      wrapperBinary = "$out/bin/gitk-go";
    in
    ''
      mv $out/bin/gitk-go ${wrappedBinary}
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
      # the package), but for now we patch the interpreter to use Nix's dynamic
      # linker and keep the usual wrapper for runtime environment setup.
      lib.optionalString stdenv.hostPlatform.isLinux ''
        patchelf --set-interpreter ${stdenv.cc.bintools.dynamicLinker} ${wrappedBinary}

        makeWrapper ${wrappedBinary} \
          ${wrapperBinary} \
          --set LD_LIBRARY_PATH ${linuxLibs} \
          ${gitPath}
      ''
    + lib.optionalString stdenv.hostPlatform.isDarwin ''
      makeWrapper ${wrappedBinary} \
        ${wrapperBinary} \
        ${gitPath}
    '';

  ldflags = [
    "-s"
    "-w"
    "-X github.com/thiagokokada/gitk-go/internal/buildinfo.version=${version}"
  ];

  meta = with lib; {
    description = "A lightweight Git history explorer written in Go";
    homepage = "https://github.com/thiagokokada/gitk-go";
    license = licenses.mit;
    mainProgram = "gitk-go";
  };
}
