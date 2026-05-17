#!/usr/bin/env bash
# apply.sh — Apply the versioned libsmartctl patches to a cloned smartmontools tree.
#
# Usage:
#   ./patches/apply.sh <version> [smartmontools-dir]
#
# Arguments:
#   version           Smartmontools tag to apply patches for (e.g. v7.5).
#   smartmontools-dir Path to the cloned smartmontools repository.
#                     Defaults to ./smartmontools in the current directory.
#
# Exit codes:
#   0  All patches applied cleanly.
#   1  Missing arguments or patch directory.
#   2  One or more patches failed to apply.
#
# Example (used by CI):
#   git clone --depth 1 --branch v7.5 https://github.com/smartmontools/smartmontools.git src
#   ./patches/apply.sh v7.5 src

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

VERSION="${1:-}"
SMART_DIR="${2:-${PWD}/smartmontools}"

if [[ -z "${VERSION}" ]]; then
  echo "Usage: $0 <version> [smartmontools-dir]" >&2
  exit 1
fi

PATCH_DIR="${SCRIPT_DIR}/${VERSION}"

if [[ ! -d "${PATCH_DIR}" ]]; then
  echo "No patches found for version ${VERSION} at ${PATCH_DIR}" >&2
  echo "Run ./patches/generate.sh ${VERSION} to create them." >&2
  exit 1
fi

if [[ ! -d "${SMART_DIR}" ]]; then
  echo "smartmontools directory not found: ${SMART_DIR}" >&2
  exit 1
fi

echo "Applying patches for ${VERSION} to ${SMART_DIR}"

FAILED=0
for patch in "${PATCH_DIR}"/*.patch; do
  echo "  Applying $(basename "${patch}") ..."
  if ! git -C "${SMART_DIR}" apply --check "${patch}" 2>/dev/null; then
    echo "  WARNING: patch does not apply cleanly: $(basename "${patch}")" >&2
    echo "  Run ./patches/generate.sh ${VERSION} to regenerate." >&2
    FAILED=1
  else
    git -C "${SMART_DIR}" apply "${patch}"
  fi
done

if [[ "${FAILED}" -ne 0 ]]; then
  echo "One or more patches failed. See warnings above." >&2
  exit 2
fi

echo "All patches applied successfully."
