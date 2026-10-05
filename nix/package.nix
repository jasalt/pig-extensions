{
  lib,
  stdenvNoCC,
  fetchgit,
  runCommand,
  go_1_27,
  git,
  cacert,
  python3,
  procps,
  pigSource ? fetchgit {
    url = "https://github.com/MichaelKinsy/PiG.git";
    rev = "6f1441ef4882e30b0681cb6119bd12474e41fd9c";
    hash = "sha256-VRXkSL+ex3bHZwa291kSIRjGXaaWamUp3/sk6KHwpYM=";
    # PiG records the revision and enumerates source files using Git.
    leaveDotGit = true;
  },
  dependencyHash ? "sha256-tfyURHy9wnbpbXqb0xEk3w1HRBX6Beh/0xhz1v8dMMo=",
}:
let
  source = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../piglet.yaml
      ../extensions
      ../LICENSE
      ../docs/provenance.md
    ];
  };
  tests = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../test/__init__.py
      ../test/integration/__init__.py
      ../test/integration/rpc.py
      ../test/integration/fused_runtime.py
      ../test/integration/savelast.py
      ../test/integration/pins_rpc.py
    ];
  };
  # A fixed-output fetch phase has network access. The actual compiler phase
  # below only sees this hash-checked, file:// Go module proxy.
  modules =
    runCommand "pig-extensions-go-modules"
      {
        nativeBuildInputs = [
          go_1_27
          git
          cacert
        ];
        outputHashMode = "recursive";
        outputHashAlgo = "sha256";
        outputHash = dependencyHash;
        impureEnvVars = lib.fetchers.proxyImpureEnvVars;
      }
      ''
        export HOME="$TMPDIR/home" GOPATH="$TMPDIR/go" GOMODCACHE="$TMPDIR/go/pkg/mod"
        export GOTOOLCHAIN=local GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
        export SSL_CERT_FILE=${cacert}/etc/ssl/certs/ca-bundle.crt
        mkdir -p "$HOME"
        cp -r ${pigSource} pig
        chmod -R u+w pig
        (cd pig; export GOWORK="$PWD/go.work"; go mod download all)
        cp -r ${source}/extensions extensions
        chmod -R u+w extensions
        for extension in extensions/*; do
          (cd "$extension"; GOWORK=off go mod download all)
        done
        mkdir -p "$out"
        cp -r "$GOMODCACHE/cache/download/." "$out/"
        chmod -R u+w "$out"
        # Transient locks, sumdb responses, and version lists aren't proxy inputs.
        rm -rf "$out/sumdb"
        find "$out" -type f ! -name '*.mod' ! -name '*.info' ! -name '*.zip' -delete
      '';
in
stdenvNoCC.mkDerivation {
  pname = "pig-extensions";
  version = "0.1.0";
  src = source;
  nativeBuildInputs = [
    go_1_27
    git
    python3
    procps
  ];
  dontStrip = true; # Piglet artifacts carry binary integrity metadata.
  dontPatchELF = true;
  buildPhase = ''
    runHook preBuild
    export HOME="$TMPDIR/home" XDG_CONFIG_HOME="$TMPDIR/config" XDG_CACHE_HOME="$TMPDIR/cache"
    export PIG_HOME="$HOME/.pig" PIG_CODING_AGENT_DIR="$HOME/.pig/agent"
    export GOPATH="$TMPDIR/go" GOMODCACHE="$TMPDIR/go/pkg/mod" GOCACHE="$TMPDIR/go-build"
    export GOTOOLCHAIN=local GOPROXY=file://${modules} GOSUMDB=off CGO_ENABLED=0
    export GOFLAGS="-trimpath -buildvcs=false"
    mkdir -p "$HOME" "$PIG_CODING_AGENT_DIR"
    cp -r ${pigSource} "$TMPDIR/pig"
    chmod -R u+w "$TMPDIR/pig"
    export PIG_SOURCE_ROOT="$TMPDIR/pig"
    (cd "$PIG_SOURCE_ROOT"; GOWORK="$PWD/go.work" go build -o "$TMPDIR/pig-bootstrap" ./cmd/pig)
    "$TMPDIR/pig-bootstrap" piglet build ./piglet.yaml --format binary --out "$TMPDIR/pig-extensions" --verbose
    runHook postBuild
  '';
  installPhase = ''
    runHook preInstall
    install -Dm755 "$TMPDIR/pig-extensions" "$out/bin/pig-extensions"
    ln -s pig-extensions "$out/bin/pig"
    mkdir -p "$out/share/doc/pig-extensions" "$out/share/doc/pig"
    cp LICENSE "$out/share/doc/pig-extensions/"
    cp docs/provenance.md "$out/share/doc/pig-extensions/"
    for license in extensions/*/LICENSE; do
      name=$(basename "$(dirname "$license")")
      install -Dm644 "$license" "$out/share/doc/pig-extensions/$name/LICENSE"
    done
    cp -r "$PIG_SOURCE_ROOT"/{LICENSE,LICENSES,NOTICE,THIRD_PARTY_NOTICES.md} "$out/share/doc/pig/"
    runHook postInstall
  '';
  doInstallCheck = true;
  installCheckPhase = ''
    runHook preInstallCheck
    "$out/bin/pig-extensions" --version
    export PIG_BIN="$out/bin/pig-extensions" PIG_FUSED=1 PYTHONDONTWRITEBYTECODE=1
    export PYTHONPATH=${tests}
    python3 -m test.integration.fused_runtime
    python3 -m test.integration.savelast
    python3 -m test.integration.pins_rpc
    runHook postInstallCheck
  '';
  passthru = { inherit modules pigSource; };
  meta = {
    description = "PiG with six extensions fused from piglet.yaml";
    homepage = "https://github.com/jasalt/pig-extensions";
    license = [
      lib.licenses.mit
      lib.licenses.isc
    ];
    platforms = [ "x86_64-linux" ];
    mainProgram = "pig-extensions";
  };
}
