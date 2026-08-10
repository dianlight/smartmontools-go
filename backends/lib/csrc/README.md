# libsmartmon_go C ABI contract

This directory contains the thin C++ wrapper (`smartmon_c_api.cpp`) that exposes the
smartmontools C++ SDK (`libsmartmon.a`) as a C ABI. The Go `backends/lib` package
consumes it at runtime via ebitengine/purego — no CGO required.

## Who builds the wrapper

The wrapper shared library (`libsmartmon_go.{so,dylib}`) is built and shipped by
[dianlight/smartmontools-sdk](https://github.com/dianlight/smartmontools-sdk)
release tarballs, pinned to a specific smartmontools-go tag (currently `v0.4.1`).
Consumers (e.g. the SRAT addon) `dlopen` the pre-built wrapper at runtime.

This repo no longer contains a wrapper build script
(`scripts/setup-lib-backend.sh` was removed in `18b27ff`); the build inputs for
the SDK are the sources in this directory.

## Exported symbols (baseline: v0.4.1)

The following symbols are the stable ABI. The Go side binds exactly these names
via `registerFuncs` in `backends/lib/lib.go`.

| Symbol | Signature |
| --- | --- |
| smartmon_init | `int smartmon_init(void)` |
| smartmon_cleanup | `void smartmon_cleanup(void)` |
| smartmon_scan_devices | `int smartmon_scan_devices(char **out_json)` |
| smartmon_get_smart_data | `int smartmon_get_smart_data(const char *device, const char *dev_type, char **out_json)` |
| smartmon_check_health | `int smartmon_check_health(const char *device, const char *dev_type, int *out_healthy)` |
| smartmon_enable_smart | `int smartmon_enable_smart(const char *device, const char *dev_type)` |
| smartmon_disable_smart | `int smartmon_disable_smart(const char *device, const char *dev_type)` |
| smartmon_run_selftest | `int smartmon_run_selftest(const char *device, const char *dev_type, const char *test_type)` |
| smartmon_abort_selftest | `int smartmon_abort_selftest(const char *device, const char *dev_type)` |
| smartmon_free_string | `void smartmon_free_string(char *s)` |
| smartmon_last_error | `const char *smartmon_last_error(void)` |

Per-function semantics and ownership rules are documented in `smartmon_c_api.h`;
heap-allocated JSON output strings must be released with `smartmon_free_string`.

## Stability rule

- The ABI must remain **backwards-compatible** across tagged releases: it is
  **add-only**. Never remove or rename an exported symbol, and never change a
  signature, without a major-version bump of smartmontools-go.
- Any ABI change must be coordinated with a new wrapper build in
  `dianlight/smartmontools-sdk` and a version bump of SRAT. Without
  coordination, `registerFuncs` in `backends/lib/lib.go` fails to bind a symbol
  and SRAT's purego backend silently falls back to the exec backend.

## Reference

- Go consumer: `backends/lib/lib.go` — `libFuncs` struct and `registerFuncs`.
- Wrapper sources: `smartmon_c_api.cpp`, `smartmon_c_api.h`, `smartmon_config.h`.
- SDK build/packaging: [dianlight/smartmontools-sdk](https://github.com/dianlight/smartmontools-sdk).
