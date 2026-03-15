#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <version>" >&2
  exit 1
fi

VERSION="$1"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${REPO_ROOT}/dist/${VERSION}"
COMMIT="$(git -C "${REPO_ROOT}" rev-parse --short HEAD)"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

if ! command -v zip >/dev/null 2>&1; then
  echo "zip is required to package Windows artifacts" >&2
  exit 1
fi

mkdir -p "${DIST_DIR}"
rm -rf "${DIST_DIR:?}/"*

platforms=(
  "linux amd64 tar.gz"
  "linux arm64 tar.gz"
  "darwin amd64 tar.gz"
  "darwin arm64 tar.gz"
  "windows amd64 zip"
  "windows arm64 zip"
)

for platform in "${platforms[@]}"; do
  read -r goos goarch archive <<<"${platform}"
  name="ccsync_${VERSION}_${goos}_${goarch}"
  stage_dir="${DIST_DIR}/${name}"
  binary_name="ccsync"

  if [[ "${goos}" == "windows" ]]; then
    binary_name="ccsync.exe"
  fi

  mkdir -p "${stage_dir}"

  GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
      -o "${stage_dir}/${binary_name}" \
      ./cmd/ccsync

  if [[ "${archive}" == "zip" ]]; then
    (
      cd "${DIST_DIR}"
      zip -rq "${name}.zip" "${name}"
    )
  else
    tar -C "${DIST_DIR}" -czf "${DIST_DIR}/${name}.tar.gz" "${name}"
  fi

  rm -rf "${stage_dir}"
done

(
  cd "${DIST_DIR}"
  sha256sum ./* > checksums.txt
)
