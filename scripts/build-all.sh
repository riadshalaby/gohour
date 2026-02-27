#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-dist}"
mkdir -p "$OUT_DIR"

TARGETS=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

echo "Building gohour for ${#TARGETS[@]} targets into: $OUT_DIR"

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
  GOOS="$os" GOARCH="$arch" go build -o "$out" .
done

echo "Done. Artifacts are in: $OUT_DIR"
