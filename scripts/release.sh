#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Usage: $0 <version> [out-dir]" >&2
  echo "Example: $0 v0.2.3 dist" >&2
  exit 1
fi

VERSION="$1"
OUT_DIR="${2:-dist}"

if [[ "${VERSION}" != v* ]]; then
  echo "Version must start with 'v' (for example: v0.2.3)." >&2
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI (gh) is required for releases." >&2
  exit 1
fi

./scripts/build-all.sh "$OUT_DIR" "$VERSION"

if git rev-parse -q --verify "refs/tags/${VERSION}" >/dev/null; then
  echo "Using existing local tag ${VERSION}"
else
  git tag "${VERSION}"
fi

git push origin "${VERSION}"

gh release create "${VERSION}" \
  "${OUT_DIR}"/gohour-* \
  "${OUT_DIR}"/SHA256SUMS \
  --title "${VERSION}" \
  --notes "Automated gohour release ${VERSION}"

echo "Release ${VERSION} created."
