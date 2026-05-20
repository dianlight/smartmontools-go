#!/usr/bin/env bash
# setup-lib-backend.sh — Download the smartmontools-sdk release and compile
# the smartmon_go wrapper shared library.
#
# Usage:
#   scripts/setup-lib-backend.sh                  # latest (auto-detect platform)
#   SMARTMON_SDK_VERSION=7.5 scripts/setup-lib-backend.sh
#
# After this script completes the wrapper library is written to:
#   backends/lib/sdk/libsmartmon_go.so   (Linux)
#   backends/lib/sdk/libsmartmon_go.dylib (macOS)
#
# Point the backend at it:
#   export SMARTMON_LIB_PATH=$(pwd)/backends/lib/sdk/libsmartmon_go.so
#   # or
#   export SMARTMON_LIB_PATH=$(pwd)/backends/lib/sdk/libsmartmon_go.dylib
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SDK_DIR="${REPO_ROOT}/backends/lib/sdk"
CPP_SRC="${REPO_ROOT}/backends/lib/csrc/smartmon_c_api.cpp"

SDK_VERSION="${SMARTMON_SDK_VERSION:-7.5}"
SDK_REPO="dianlight/smartmontools-sdk"
SDK_RELEASE_URL="https://github.com/${SDK_REPO}/releases/download/v${SDK_VERSION}"

# ── Platform detection ──────────────────────────────────────────────────────
OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
  Linux)  OS_TAG="linux"  ;;
  Darwin) OS_TAG="darwin" ;;
  *)
    echo "error: unsupported OS '${OS}'. Only Linux and macOS are supported." >&2
    exit 1
    ;;
esac

# Map uname -m values to SDK asset arch names.
case "${ARCH}" in
  x86_64)          ARCH_TAG="amd64"   ;;
  aarch64 | arm64) ARCH_TAG="aarch64" ;;
  *)
    echo "error: unsupported architecture '${ARCH}'." >&2
    exit 1
    ;;
esac

ASSET="libsmartmon-${SDK_VERSION}-${OS_TAG}-${ARCH_TAG}.tar.gz"
DOWNLOAD_URL="${SDK_RELEASE_URL}/${ASSET}"

# ── Download ────────────────────────────────────────────────────────────────
mkdir -p "${SDK_DIR}"
ARCHIVE="${SDK_DIR}/${ASSET}"

if [ ! -f "${ARCHIVE}" ]; then
  echo "Downloading ${ASSET}..."
  if command -v gh &>/dev/null; then
    gh release download "v${SDK_VERSION}" \
      --repo "${SDK_REPO}" \
      --pattern "${ASSET}" \
      --dir "${SDK_DIR}" \
      --clobber
  elif command -v curl &>/dev/null; then
    curl -fsSL -o "${ARCHIVE}" "${DOWNLOAD_URL}"
  else
    echo "error: neither 'gh' nor 'curl' is available." >&2
    exit 1
  fi
else
  echo "Archive already present: ${ARCHIVE}"
fi

# ── Extract ─────────────────────────────────────────────────────────────────
echo "Extracting SDK..."
tar -xzf "${ARCHIVE}" -C "${SDK_DIR}" --strip-components=0

INCLUDE_DIR="${SDK_DIR}/include"
LIB_DIR="${SDK_DIR}/lib"

if [ ! -d "${INCLUDE_DIR}" ] || [ ! -f "${LIB_DIR}/libsmartmon.a" ]; then
  echo "error: expected ${INCLUDE_DIR} and ${LIB_DIR}/libsmartmon.a after extraction." >&2
  exit 1
fi

# ── Compile wrapper ─────────────────────────────────────────────────────────
echo "Compiling smartmon_go wrapper shared library..."

CXX="${CXX:-g++}"
CXXFLAGS_COMMON="-std=c++17 -fPIC -O2 -I${INCLUDE_DIR}"

case "${OS_TAG}" in
  linux)
    OUT="${SDK_DIR}/libsmartmon_go.so"
    "${CXX}" ${CXXFLAGS_COMMON} \
      -shared \
      -o "${OUT}" \
      "${CPP_SRC}" \
      -L"${LIB_DIR}" -lsmartmon \
      -lstdc++ -lm
    ;;
  darwin)
    OUT="${SDK_DIR}/libsmartmon_go.dylib"
    "${CXX}" ${CXXFLAGS_COMMON} \
      -dynamiclib \
      -o "${OUT}" \
      "${CPP_SRC}" \
      -L"${LIB_DIR}" -lsmartmon \
      -lstdc++ -lm \
      -framework CoreFoundation \
      -framework IOKit
    ;;
esac

echo ""
echo "✓ Wrapper library built: ${OUT}"
echo ""
echo "To use with the lib backend, set:"
echo "  export SMARTMON_LIB_PATH=${OUT}"
echo ""
echo "Or pass it directly:"
echo '  lib, err := libbackend.New(libbackend.WithLibraryPath("'"${OUT}"'"))'
