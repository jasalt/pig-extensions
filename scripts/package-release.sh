#!/usr/bin/env bash
# Package the Nix output without modifying the Piglet integrity-protected binary.
set -euo pipefail

if [[ $# != 3 ]]; then
  echo "Usage: $0 NIX_OUTPUT RELEASE_TAG DIST_DIRECTORY" >&2
  exit 2
fi
output=$(realpath "$1")
tag=$2
dist=$3
if [[ ! $tag =~ ^v[0-9][A-Za-z0-9._+-]*$ ]]; then
  echo "Release tag must start with v and a digit and contain only letters, digits, ., _, +, or -." >&2
  exit 2
fi
mkdir -p "$dist"
dist=$(realpath "$dist")
name="pig-extensions-${tag}-linux-x86_64"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/$name/bin" "$work/$name/share"
install -m755 "$output/bin/pig-extensions" "$work/$name/bin/pig-extensions"
ln -s pig-extensions "$work/$name/bin/pig"
cp -R "$output/share/doc" "$work/$name/share/"
# Nix store directories are read-only; make the copy removable and normalize
# archive permissions without touching executable bytes.
find "$work/$name/share" -type d -exec chmod 755 {} +
find "$work/$name/share" -type f -exec chmod 644 {} +

# Reject binaries needing a dynamic loader or shared libraries. A Nix-linked
# executable would not be a usable standalone release on an ordinary Linux host.
readelf -l "$work/$name/bin/pig-extensions" > "$work/program-headers"
readelf -d "$work/$name/bin/pig-extensions" > "$work/dynamic-section"
if grep -Eq 'INTERP' "$work/program-headers" || grep -Eq '\(NEEDED\)' "$work/dynamic-section"; then
  echo "Release binary is not static; refusing to publish a Nix-dependent executable." >&2
  exit 1
fi
mkdir -p "$work/home" "$work/config" "$work/cache"
env -i PATH=/usr/bin:/bin HOME="$work/home" \
  XDG_CONFIG_HOME="$work/config" XDG_CACHE_HOME="$work/cache" \
  PIG_HOME="$work/home/.pig" PIG_CODING_AGENT_DIR="$work/home/.pig/agent" \
  "$work/$name/bin/pig-extensions" --version

# Record what was released: the extension tag is distinct from PiG's version.
printf '%s\n' "$tag" > "$work/$name/RELEASE_TAG"
tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner \
  -C "$work" -cf - "$name" | gzip -n > "$dist/$name.tar.gz"
(cd "$dist"; sha256sum "$name.tar.gz" > "$name.tar.gz.sha256")
echo "Created $dist/$name.tar.gz and checksum"
