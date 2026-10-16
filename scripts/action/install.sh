#!/usr/bin/env bash
set -euo pipefail

if [[ "${RUNNER_OS:-}/${RUNNER_ARCH:-}" != Linux/X64 ]]; then
  echo 'Linebind setup supports Linux x64 runners; use the release archives on other platforms.' >&2
  exit 2
fi
if [[ ! "${LINEBIND_VERSION:-}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
   [[ ! "${LINEBIND_SHA256:-}" =~ ^[0-9a-f]{64}$ ]]; then
  echo 'Provide an exact vX.Y.Z version and a lowercase SHA-256 checksum.' >&2
  exit 2
fi
: "${RUNNER_TEMP:?}" "${GITHUB_PATH:?}" "${GITHUB_OUTPUT:?}"
install_dir=$(mktemp -d "$RUNNER_TEMP/linebind.XXXXXXXX")
trap 'rm -rf "$install_dir"' EXIT
archive="$install_dir/archive.tar.gz"
asset="linebind_${LINEBIND_VERSION#v}_linux_amd64.tar.gz"
curl --fail --silent --show-error --location --retry 3 \
  "https://github.com/mchl-schrdng/linebind/releases/download/$LINEBIND_VERSION/$asset" \
  --output "$archive"
actual=$(shasum -a 256 "$archive")
if [[ "${actual%% *}" != "$LINEBIND_SHA256" ]]; then
  echo 'Linebind archive checksum mismatch; refusing to extract or execute it.' >&2
  exit 2
fi
tar -xzf "$archive" -C "$install_dir" linebind
test -f "$install_dir/linebind" && test ! -L "$install_dir/linebind"
"$install_dir/linebind" --version
rm "$archive"
printf '%s\n' "$install_dir" >> "$GITHUB_PATH"
printf 'binary=%s/linebind\n' "$install_dir" >> "$GITHUB_OUTPUT"
trap - EXIT
