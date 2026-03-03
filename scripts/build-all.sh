#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-dist}"
VERSION="${2:-${GOHOUR_VERSION:-}}"

if [[ -z "$VERSION" ]]; then
  if VERSION="$(git describe --tags --always --dirty 2>/dev/null)"; then
    :
  else
    VERSION="dev"
  fi
fi

mkdir -p "$OUT_DIR"

TARGETS=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

LDFLAGS="-X gohour/cmd.Version=${VERSION}"

echo "Building gohour ${VERSION} for ${#TARGETS[@]} targets into: $OUT_DIR"

for target in "${TARGETS[@]}"; do
  set -- $target
  os="$1"
  arch="$2"

  ext=""
  if [[ "$os" == "windows" ]]; then
    ext=".exe"
  fi

  out="$OUT_DIR/gohour-${os}-${arch}${ext}"
  echo "-> GOOS=$os GOARCH=$arch => $out"
  GOOS="$os" GOARCH="$arch" go build -ldflags "$LDFLAGS" -o "$out" .
done

pushd "$OUT_DIR" >/dev/null
artifacts=(gohour-*)
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "${artifacts[@]}" > SHA256SUMS
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "${artifacts[@]}" > SHA256SUMS
else
  echo "No SHA256 tool found (expected sha256sum or shasum)." >&2
  exit 1
fi
popd >/dev/null

echo "Done. Artifacts are in: $OUT_DIR"
echo "Checksums written to: $OUT_DIR/SHA256SUMS"
