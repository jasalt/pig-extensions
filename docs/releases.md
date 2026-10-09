# Tagged binary releases

GitHub Actions builds a standalone **Linux x86-64** fused binary when a tag
matching `v*` is pushed. Use tags such as `v0.1.0` or `v0.2.0-rc.1`; packaging
requires `v` followed by a digit and only letters, digits, `.`, `_`, `+`, or `-`.
No macOS, Windows, or ARM binaries are currently built.

## Publish a release

Commit and push the reviewed source first, then tag that commit:

```sh
git tag -a v0.1.0 -m 'pig-extensions v0.1.0'
git push github v0.1.0
```

The workflow in `.github/workflows/release.yml`:

1. Checks out the tagged source and installs Nix.
2. Runs `nix build . --print-build-logs`, using the committed flake lock and
   pinned PiG/module hashes. The package's install checks run on a fresh build;
   Nix can also reuse an already checked store result.
3. Copies the executable without stripping or modifying its integrity metadata,
   includes the `pig` symlink and all installed license/provenance notices, and
   rejects an ELF dynamic loader or shared-library dependency.
4. Runs the copied binary's `--version` with an isolated, empty home and settings.
5. Creates a tar archive and SHA-256 checksum, then creates a GitHub Release with
   generated notes and uploads the assets. An existing release is reused;
   rerunning replaces same-named assets. Do not move published tags.

The job uses the repository's `GITHUB_TOKEN` with `contents: write`; no separate
release secret is required. Repository/organization policy must permit Actions
and this permission. No credentials, real alerts, or reset redemption are used
by the build checks.

Assets are named:

- `pig-extensions-v0.1.0-linux-x86_64.tar.gz`
- `pig-extensions-v0.1.0-linux-x86_64.tar.gz.sha256`

The archive has `bin/pig-extensions`, `bin/pig`, `share/doc/`, and `RELEASE_TAG`
inside a versioned directory. The tag identifies this extension release;
`pig-extensions --version` still reports the pinned **upstream PiG version**.
The Nix derivation's package version remains independent of the tag.

## Download and run

Download both assets from the corresponding GitHub Release, then:

```sh
sha256sum -c pig-extensions-v0.1.0-linux-x86_64.tar.gz.sha256
tar -xzf pig-extensions-v0.1.0-linux-x86_64.tar.gz
./pig-extensions-v0.1.0-linux-x86_64/bin/pig-extensions --version
```

Nix and Go are not required on the destination machine. Runtime provider
credentials, shell tools, browser, and graphics-capable terminal remain external.
The static ELF check and version smoke test are not full cross-distribution,
interactive graphics, or desktop-browser qualification. See
[nix.md](nix.md#runtime-scope-and-verification) for the functional build-check
scope and [validation.md](validation.md) for broader evidence.

## Local packaging verification

On Linux x86-64 with Nix, Bash, GNU coreutils/tar, gzip, and binutils:

```sh
nix build .
bash scripts/package-release.sh result v0.1.0 dist
(cd dist; sha256sum -c *.sha256)
```

Packaging uses a temporary isolated home, removes it afterward, and normalizes
archive ordering, timestamps, ownership, and permissions. Repackaging the same
Nix output with the same tag produces identical archive bytes. This is not a
claim that independent PiG compiler builds are bit-for-bit reproducible.
