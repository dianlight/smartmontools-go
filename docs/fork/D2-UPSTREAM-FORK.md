# D2: Upstream Fork with CI Sync

## Overview

This directory documents the **D2 strategy** for building `libsmartctl` — a
permanent fork of smartmontools at `github.com/dianlight/smartmontools-sdk`
that includes the C API shim directly in its source tree. A GitHub Action
(`sync-upstream.yml`) rebases the fork onto upstream weekly.

## Fork Repository Structure

The fork at `github.com/dianlight/smartmontools-sdk` contains the
upstream smartmontools source with the following additions:

```
smartmontools-sdk/
├── src/
│   ├── libsmartctl.h          # C API header (added)
│   ├── libsmartctl.cpp        # C API implementation (added)
│   └── Makefile.am            # +libsmartctl.la target (modified)
├── .github/workflows/
│   ├── build-libsmartctl.yml  # Build shared library per platform
│   └── sync-upstream.yml      # Weekly rebase onto upstream
└── ...                        # (rest is upstream smartmontools)
```

## Files Added to the Fork

### `src/libsmartctl.h`

The public C API header. See the patch at
`../patches/RELEASE_7_5/0001-add-libsmartctl-h.patch` for the authoritative
version, or copy the file below directly into the fork.

### `src/libsmartctl.cpp`

The C API implementation that wraps smartmontools internals. See the patch at
`../patches/RELEASE_7_5/0002-add-libsmartctl-cpp.patch` for the authoritative
version.

### `src/Makefile.am` (modified)

Adds the conditional `libsmartctl.la` target. See the patch at
`../patches/RELEASE_7_5/0003-modify-makefile-am.patch` for the diff.

### `src/smartctl.cpp` (modified)

Wraps `main()` with `#ifndef BUILDING_LIBSMARTCTL`. See the patch at
`../patches/RELEASE_7_5/0004-exclude-main-on-lib-build.patch` for the diff.

## CI Workflows

### sync-upstream.yml (in the fork)

Rebases the fork's `main` branch onto `smartmontools/smartmontools:main` weekly.
On rebase conflict, files a GitHub issue for manual resolution.

A copy of this workflow is maintained in this repo at:
`.github/workflows/sync-upstream.yml`

### build-libsmartctl.yml (in this repo)

Builds `libsmartctl.so` / `libsmartctl.dylib` for Linux (amd64, arm64) and
macOS (arm64, amd64) from the fork. Uploads artifacts for release attachment.

Located at: `.github/workflows/build-libsmartctl.yml`

## Setup Instructions

### 1. Create the Fork

```bash
# Fork upstream
gh repo fork smartmontools/smartmontools \
  --org dianlight \
  --name smartmontools-sdk \
  --default-branch-only

# Clone the fork
git clone git@github.com:dianlight/smartmontools-sdk.git
cd smartmontools-sdk
```

### 2. Add Upstream Remote

```bash
git remote add upstream https://github.com/smartmontools/smartmontools.git
git fetch upstream main
```

### 3. Apply C API Changes

Apply the D1 patches as commits on top of the fork:

```bash
# Start from the latest upstream tag
git checkout -b add-libsmartctl upstream/RELEASE_7_5

# Apply patches as individual commits
git am ../smartmontools-go/patches/RELEASE_7_5/*.patch

# Verify the build
./autogen.sh
./configure --enable-shared --disable-static --enable-libsmartctl \
  CFLAGS="-fPIC" CXXFLAGS="-fPIC -DBUILDING_LIBSMARTCTL"
make -j$(nproc)
```

### 4. Add CI Workflows

Copy the CI workflows from this repo into the fork:

```bash
mkdir -p .github/workflows
cp ../smartmontools-go/.github/workflows/sync-upstream.yml .github/workflows/
```

### 5. Push and Enable Actions

```bash
git push origin add-libsmartctl:main
```

Go to the fork's Settings → Actions → General and ensure "Allow all actions"
is enabled.

### 6. Configure Repository Variables

Set the following variable in this repository (Settings → Secrets and
variables → Actions → Variables):

| Variable | Value |
|----------|-------|
| `FORK_REPO` | `dianlight/smartmontools-sdk` |

### 7. Create a PAT for the Sync Workflow

The `sync-upstream.yml` workflow (in the fork) needs a Personal Access Token
with `repo` scope to force-push. Create one and add it as a secret:

```bash
gh secret set PAT --body "<your-pat>" --repo dianlight/smartmontools-sdk
```

## Troubleshooting

### Rebase Conflicts

When the weekly sync encounters a conflict:

1. Check the issue filed under the `upstream-sync` label
2. Fetch upstream: `git fetch upstream main`
3. Rebase manually: `git rebase upstream/main`
4. Resolve conflicts, focusing on `Makefile.am` and `smartctl.cpp`
5. Force-push: `git push origin main --force-with-lease`

### Build Failures

If the build fails after a successful sync:

1. Check if upstream changed the build system (autotools version, dependencies)
2. Verify `--enable-libsmartctl` is still recognized by `./configure --help`
3. Check if internal headers referenced by `libsmartctl.cpp` have moved

### Library Not Found by Go Backend

If the Go `LibBackend` cannot find the library:

1. Install the `.so`/`.dylib` to a standard path:
   ```bash
   sudo cp libsmartctl.so* /usr/local/lib/
   sudo ldconfig  # Linux only
   ```
2. Or set `LD_LIBRARY_PATH` / `DYLD_LIBRARY_PATH`:
   ```bash
   export LD_LIBRARY_PATH=/path/to/lib:$LD_LIBRARY_PATH
   ```
3. Or use `WithLibraryPath` option when creating the backend:
   ```go
   b, _ := lib.New(lib.WithLibraryPath("/path/to/libsmartctl.so"))
   ```
