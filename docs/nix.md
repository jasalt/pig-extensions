# Nix and Home Manager

The flake packages the root `piglet.yaml` as a **fused Piglet Binary** on
`x86_64-linux`. It installs `pig-extensions` and a `pig` symlink to that binary.
All recipe members are compiled in. It does not install extension source
into your PiG settings, change credentials, migrate state, or run an agent
session during Home Manager activation.

## Build without local checkouts

With Nix and flakes enabled and an Internet connection:

```sh
nix build github:jasalt/pig-extensions
./result/bin/pig-extensions --version
# To launch an interactive session instead:
nix run github:jasalt/pig-extensions
```

These remote examples require the commit containing this flake to be available
on GitHub. This implementation does not publish it. Until then, use
`nix build .` from this repository, or the local override below.

No existing `pig`, Go installation, `../pig-upstream`, or module cache is needed.
Nix downloads pinned nixpkgs/toolchains, the reviewed PiG Git revision, and a
hash-checked Go module proxy. It builds stock PiG as a bootstrap tool, then runs
`pig piglet build ./piglet.yaml --format binary` with an absolute
`PIG_SOURCE_ROOT`. The compiler phase is offline; dependency fetching uses
network-enabled fixed-output derivations. Repeated builds reuse the Nix store.
No `--impure`, sandbox disabling, or networked activation script is needed.

Builds may take several minutes and substantial disk space. The package uses
Go 1.27.1; extension language floors remain Go 1.26 and SDK requirements remain
v0.3.1. Source/module pins are in `nix/package.nix`; nixpkgs is in `flake.lock`.
PiG Git metadata is retained because its builder records the actual source
revision. After `fetchgit`, packaging keeps a shallow, detached repository with
only the original pinned commit and its tree, repacked without object reuse.
Remote refs, tags, and fetch metadata are discarded. GitHub's pack encoding and
remote refs can change even when the source revision does not. No upstream
source patches or synthetic commits are used.

## Home Manager: recommended module

In your Home Manager **flake.nix**, add the input:

```nix
inputs.pig-extensions.url = "github:jasalt/pig-extensions";
```

Keep this input's own nixpkgs pin by default: it supplies the tested Go toolchain.
There is no need to add `inputs.nixpkgs.follows`. Your consumer flake lock pins
this repository too.

Bind `pig-extensions` in your existing `outputs` arguments and import its module:

```nix
outputs = { nixpkgs, home-manager, pig-extensions, ... }: {
  homeConfigurations."user@lima-default" = home-manager.lib.homeManagerConfiguration {
    pkgs = import nixpkgs { system = "x86_64-linux"; };
    modules = [
      # Keep your other modules/overlays unchanged.
      pig-extensions.homeManagerModules.default
      ./home.nix
    ];
  };
};
```

Then add this to **home.nix**:

```nix
programs.pig-extensions.enable = true;
```

From the Home Manager flake directory:

```sh
nix flake lock
home-manager switch --flake '.#user@lima-default'
type -a pig pig-extensions
pig-extensions --version
```

A fresh switch automatically fetches and builds the full package if it is not
cached. Nix must already be installed; this is not a Nix installer. There is no
requirement for either source repository to exist locally.

Your current configuration prepends `$HOME/go/bin` to PATH. An older `pig` there
may still win over the Nix profile. Use `pig-extensions` explicitly, or adjust
PATH/remove that older installation yourself. This module never overwrites it.
Do not add a second package providing `bin/pig` to the same Home Manager profile.

### Package-only alternative

If you do not want the module, pass the input through `extraSpecialArgs`:

```nix
# In homeManagerConfiguration:
extraSpecialArgs = { inherit pig-extensions; };
```

```nix
# home.nix:
{ pkgs, pig-extensions, ... }: {
  home.packages = [
    pig-extensions.packages.${pkgs.stdenv.hostPlatform.system}.default
  ];
}
```

Use either the module or the package-only method, not both.

## Local development and updates

From this repository:

```sh
nix build .
nix flake check
```

New files must be Git-tracked for Git-backed local flakes. `path:$PWD` is useful
while developing untracked changes. Builds only consume the selected recipe,
extension sources, notices, and test fixtures, not ambient PiG settings.

To test this checkout from the Home Manager directory without publishing it:

```sh
home-manager switch --flake '.#user@lima-default' \
  --override-input pig-extensions path:/absolute/path/to/pig-extensions \
  --no-write-lock-file
```

Normal switches use the remote input and lock again. There is deliberately no
implicit detection of sibling checkouts: local modifications must not silently
change a supposedly pinned deployment.

To update the consumer's package after publishing a reviewed change:

```sh
nix flake update pig-extensions
home-manager switch --flake '.#user@lima-default'
```

To change the composition, edit `piglet.yaml` in a local checkout/fork and
rebuild. A recipe/source change is a new immutable binary, not `/reload`.
For ordinary extension development, keep using stock PiG with `-e`.

Maintainers changing dependencies should set `dependencyHash` in
`nix/package.nix` to `lib.fakeHash`, build, review the fetched dependency changes,
and replace it with Nix's reported hash. A PiG revision update similarly needs
its `fetchgit` hash refreshed. Never disable hash checking. A nixpkgs/toolchain
update is separate from updating the PiG revision. The package accepts
`pigSource` and `dependencyHash` overrides for deliberate development; a local
PiG source override must retain real Git metadata and a compatible workspace.

## Runtime scope and verification

The package build runs `--version`, checks fused registration/no runner child,
and executes the existing real RPC `/savelast` and `/pin` tests. All use isolated
homes and synthetic sessions without model requests, alerts, or reset redemption.
Registration is only placement evidence; those two command suites supply the
functional checks. This is not full-suite, desktop-browser, or graphics evidence.

At runtime normal PiG settings/auth remain external and writable. Configure
providers via the usual PiG login/settings workflow. The scheduler and Pushover
notifier are included and active; no credentials are shipped. Nix does not
provision a desktop browser, graphics-capable terminal, or model service.
The binary can use shell tools already on your PATH. No Go toolchain is needed
just to run these fused extensions. See [in-process.md](in-process.md) for shared
process behavior and [validation.md](validation.md) for broader qualification.

License/provenance files ship under `share/doc/pig-extensions` and `share/doc/pig`.
The Nix packaging changes installation/build mechanics, not extension behavior.

### Fresh-fetch regression (current PiG pin)

The original retained-`.git` fetch of PiG
`6f1441ef4882e30b0681cb6119bd12474e41fd9c` hashed to
`sha256-VRXkSL+ex3bHZwa291kSIRjGXaaWamUp3/sk6KHwpYM=` locally but
`sha256-O2kFvC2ElIvKDojA8rRwj2KZRTodS46enPJS452+7H8=` on a fresh fetch.
Comparison found differences only in `.git/objects` packfiles and their index;
the checked-out source was unchanged.

The normalized source NAR hash is
`sha256-eWWVRYDhjDeOOScigfej0OTsR1cwYp9i1QMpX1L3jAU=`. Independent fetches
with different output names, including one with an injected tag before
normalization, matched it; normalizing the original cached pack matched too.
The full package build passed all four install-check
suites (fused runtime, savelast, pins, and session migration), reporting
`0.4.0+1.0.0`. `nix flake check` passed. The module proxy hash remains
`sha256-tfyURHy9wnbpbXqb0xEk3w1HRBX6Beh/0xhz1v8dMMo=`.

### Recorded packaging validation (previous PiG pin)

Validated on Linux x86_64 with Nix 2.34.8 and `sandbox = true`:

- Fresh source/module fetches from HTTPS; no sibling sources or existing Go
  module cache used. Compiler/test derivation succeeded with only the local
  hash-checked module proxy.
- PiG revision `0e6ed0048282a15531ea1652a5833de4da4dd1a7`; source NAR SHA-256
  `sha256-j3nk6kSst6/jbZVFe0gsTMk9D4f1BOv4QCNqslIVA28=`.
- Module proxy NAR SHA-256
  `sha256-aqEH0dakggKBNxCtLJxl3OiljA4OanducxZFj/pfoFg=`.
- `nix flake check`: build plus disabled/enabled/package-override module checks.
- Installed binary reports `0.3.1+0.87.1`; all three install-check suites pass.
- A minimal real Home Manager configuration evaluated to an activation
  derivation using Home Manager `a26f7158fbc4aa740fcc7fa79a01e1ed29910bc2`.
  No Home Manager activation or global installation was performed.

This is build/functional evidence for the recorded pins, not a guarantee for
arbitrary PiG/nixpkgs overrides or other platforms.
