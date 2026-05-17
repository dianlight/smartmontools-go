#!/usr/bin/env bash
# generate.sh — Generate or regenerate libsmartctl patches for a smartmontools version.
#
# Usage:
#   ./patches/generate.sh <version> <patched-smartmontools-dir>
#
# Arguments:
#   version                   Smartmontools tag (e.g. v7.5).
#   patched-smartmontools-dir Path to a cloned repo with the libsmartctl changes
#                             staged or committed on top of the upstream tag.
#
# The script creates ./patches/<version>/ and writes the four patch files.
#
# Typical workflow when upstream changes break existing patches:
#
#   1. Clone the new upstream tag:
#        git clone --depth 1 --branch v7.6 \
#          https://github.com/smartmontools/smartmontools.git work/sm-v7.6
#
#   2. Manually apply your changes in work/sm-v7.6 (add libsmartctl.h,
#      libsmartctl.cpp, edit Makefile.am, guard main() in smartctl.cpp).
#
#   3. Stage all changes:
#        git -C work/sm-v7.6 add -A
#
#   4. Run this script:
#        ./patches/generate.sh v7.6 work/sm-v7.6
#
# The resulting patches can be applied with ./patches/apply.sh v7.6.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

VERSION="${1:-}"
SMART_DIR="${2:-}"

if [[ -z "${VERSION}" || -z "${SMART_DIR}" ]]; then
  echo "Usage: $0 <version> <patched-smartmontools-dir>" >&2
  exit 1
fi

if [[ ! -d "${SMART_DIR}/.git" ]]; then
  echo "Not a git repository: ${SMART_DIR}" >&2
  exit 1
fi

OUT_DIR="${SCRIPT_DIR}/${VERSION}"
mkdir -p "${OUT_DIR}"

echo "Generating patches for ${VERSION} from ${SMART_DIR} → ${OUT_DIR}"

# Generate individual patches for each logical change.
# We rely on the convention that each change is a separate commit on the
# working branch on top of the upstream tag.

git -C "${SMART_DIR}" format-patch \
  --output-directory "${OUT_DIR}" \
  --numbered \
  "HEAD~4..HEAD" \
  -- \
  src/libsmartctl.h \
  src/libsmartctl.cpp \
  src/Makefile.am \
  src/smartctl.cpp

# Rename patches to stable names so apply.sh can find them.
i=1
for f in "${OUT_DIR}"/*.patch; do
  base=$(basename "${f}")
  case "${i}" in
    1) target="0001-add-libsmartctl-h.patch" ;;
    2) target="0002-add-libsmartctl-cpp.patch" ;;
    3) target="0003-modify-makefile-am.patch" ;;
    4) target="0004-exclude-main-on-lib-build.patch" ;;
    *) target="${base}" ;;
  esac
  mv "${f}" "${OUT_DIR}/${target}"
  echo "  wrote ${target}"
  i=$((i + 1))
done

echo "Patch generation complete. Verify with: ./patches/apply.sh ${VERSION}"
