# Analysis Report: smart-sniffer/agent vs smartmontools-go

**Source analyzed:** [DAB-LABS/smart-sniffer — agent/](https://github.com/DAB-LABS/smart-sniffer/tree/main/agent)  
**Date:** May 14, 2026  
**Purpose:** Identify solutions and optimizations from smart-sniffer that can be ported to this library.

---

## Overview

`smart-sniffer/agent` is a self-contained REST API daemon (~1k LOC in `main.go` alone) that wraps `smartctl` and exposes drive data over HTTP, primarily targeting Home Assistant integration. `smartmontools-go` is a reusable Go library wrapping the same underlying tool. The two projects share many low-level concerns, and several patterns from smart-sniffer are either missing or less robust in this library.

---

## Finding 1: Smartctl Binary Resolution

**Priority:** High  
**Status:** ✅ Resolved — implemented in [`helpers.go`](../helpers.go) (`resolveSmartctlPath`, `smartctlSearchPaths`) and [`client.go`](../client.go) (`NewClient`). Tests added in [`helpers_test.go`](../helpers_test.go).

### smart-sniffer approach

Maintains a `smartctlSearchPaths` slice with 12+ platform-specific paths tried in order after `PATH`:

| Platform                       | Path                                                     |
| ------------------------------ | -------------------------------------------------------- |
| Synology (native)              | `/usr/syno/bin/smartctl`                                 |
| Synology (QPKG alternate)      | `/volume1/@appstore/smartmontools/usr/sbin/smartctl`     |
| QNAP (Entware)                 | `/opt/sbin/smartctl`                                     |
| QNAP (QPKG)                    | `/share/CACHEDEV1_DATA/.qpkg/smartmontools/bin/smartctl` |
| Standard Linux                 | `/usr/sbin/smartctl`                                     |
| FreeBSD / TrueNAS CORE         | `/usr/local/sbin/smartctl`                               |
| macOS Homebrew (Intel)         | `/usr/local/bin/smartctl`                                |
| macOS Homebrew (Apple Silicon) | `/opt/homebrew/bin/smartctl`                             |
| MacPorts                       | `/opt/local/sbin/smartctl`                               |
| NixOS                          | `/run/current-system/sw/sbin/smartctl`                   |

It also produces actionable error messages that include install instructions for each platform when no binary is found.

### Current state in smartmontools-go

`NewClient` calls only `exec.LookPath("smartctl")`. If `smartctl` is not in `PATH` the client fails to construct, requiring users on NAS platforms to always pass `WithSmartctlPath(...)` explicitly.

### Recommendation

Add an internal fallback search list equivalent to smart-sniffer's `smartctlSearchPaths`. The existing `WithSmartctlPath` option continues to take precedence. The platform-specific error message template from smart-sniffer can be reused verbatim since it is purely informational.

---

## Finding 2: SAT Protocol Automatic Fallback

**Priority:** High  
**Status:** ✅ Resolved — implemented in [`client.go`](../client.go) (`retrySATFallback`, updated `GetSMARTInfo`). Tests added in [`client_helpers_test.go`](../client_helpers_test.go).

### smart-sniffer approach

Inside `fetchDriveInfo`, when `smartctl -a <device>` exits with any execution failure bits set (bits 0–2 of the exit code), it immediately retries with the explicit `-d sat` flag. On success, the `sat` protocol is written to `protocolCache` for all subsequent calls to that device. This transparently handles:

- Synology `/dev/sata*` paths
- Many USB-to-SATA bridges where the default protocol detection fails
- RAID controllers exposing passthrough devices

### Current state in smartmontools-go

`GetSMARTInfo` handles only exit bit 1 (standby, value `2`). Other execution failure exit codes fall through to a generic error return. The `deviceTypeCache` is seeded from `drivedb.h` (USB bridge entries), but if the cached type fails at runtime there is no recovery path.

### Recommendation

In `GetSMARTInfo`, when the exit code has execution failure bits (bits 0–2, mask `0x07`) set and the cached device type is not already `sat`, retry with `-d sat`. On success, update the device type cache. This is a targeted, low-risk addition isolated to the retry branch and does not affect the hot path for correctly detected devices.

---

## Finding 3: Full Exit Code Bit Decomposition

**Priority:** Medium  
**Status:** ✅ Resolved — `ExitCodeInfo` struct added to [`types.go`](../types.go); field populated in `checkSmartStatus` and `logHealthBits` (per-device deduplication via `healthBitsCache`) added in [`client.go`](../client.go). Tests added in [`smartstatus_test.go`](../smartstatus_test.go).

### smart-sniffer approach

`decodeExecBits()` maps all 8 exit code bits to human-readable labels. The bits are also split into two semantic groups that drive different logging behaviour:

| Bits | Meaning                                                                    | Action                                                             |
| ---- | -------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| 0–2  | Execution failures (binary missing, permission denied, device open failed) | Always log as WARNING                                              |
| 3–7  | Drive health flags (SMART failing, errors in log, pre-failure attributes)  | Log once per device, suppressed on repeat via `lastHealthBits` map |

The per-device `lastHealthBits` map means that a drive in a degraded-but-stable state produces one log line, not one per poll cycle.

### Current state in smartmontools-go

Only bit 1 (standby) is handled. All other non-zero exit codes surface as opaque errors. The `messageCache` TTL deduplication is close but deduplicates by message string rather than by health-bit pattern, so a rotating message (e.g., temperature fluctuation) would never be suppressed.

### Recommendation

Decompose the exit code into `ExecBits` (0–2) and `HealthBits` (3–7). Consider adding an `ExitCodeInfo` field to `SMARTInfo` so consumers can inspect severity programmatically without parsing error strings. The per-device health-bit deduplication pattern from smart-sniffer is directly portable and would complement the existing `messageCache`.

---

## Finding 4: `--scan-open` → `--scan` Fallback in `ScanDevices`

**Priority:** Medium  
**Status:** ✅ Resolved — `ScanDevices` now retries with `--scan --json` when `--scan-open --json` fails. Implemented in [`client.go`](../client.go). Tests added in [`smartmontools_test.go`](../smartmontools_test.go) (`TestScanDevicesFallback`, updated `TestScanDevicesError`).

### smart-sniffer approach

`Refresh()` tries `--scan-open` (which wakes sleeping drives for a baseline read) and falls back to plain `--scan` if `--scan-open` fails. The `--scan-open` path is further gated to the first poll only so that subsequent polls do not unnecessarily spin up hibernating drives.

### Current state in smartmontools-go

`ScanDevices` unconditionally uses `--scan-open` with no fallback. On environments where `--scan-open` is unsupported (e.g., certain container sandboxes, older kernels, or smartctl < 7.2), the method returns an error rather than degrading gracefully to `--scan`.

### Recommendation

Add a `--scan` retry on failure inside `ScanDevices`. This is a single additional command invocation on the error path and adds no overhead to the common case.

---

## Finding 5: Drive Discovery / Protocol Probe Mode

**Priority:** Medium  
**Status:** ✅ Resolved — `DiscoverDevices(ctx) ([]DiscoveryResult, error)` added to `SmartClient` interface; `DiscoveryResult` struct added to [`types.go`](../types.go); implementation in [`client.go`](../client.go). Tests added in [`smartmontools_test.go`](../smartmontools_test.go) (`TestDiscoverDevices_*`).

### smart-sniffer approach

`RunDiscover` (in `discover.go`) is a diagnostic tool that:

1. Scans all drives via `--scan-open` (with `--scan` fallback)
2. Probes each with its auto-detected protocol
3. Attempts SAT fallback per drive if the initial read fails
4. Reports per drive: model, serial, detected protocol, SMART readable (yes/no), SAT fallback required (yes/no)
5. Generates ready-to-paste `device_overrides` config entries for drives that need manual intervention
6. Optionally writes those entries directly to the config file after user confirmation

### Current state in smartmontools-go

There is no diagnostic discovery mode. Users who encounter protocol detection failures have no guided path to resolution.

### Recommendation

Add an optional `DiscoverDevices(ctx context.Context) ([]DiscoveryResult, error)` method to the `SmartClient` interface or as a standalone function. `DiscoveryResult` would carry: `DevicePath`, `DetectedProtocol`, `SMARTReadable bool`, `SATFallbackRequired bool`, `Model`, `Serial`. This is additive and does not change any existing method signatures.

---

## Finding 6: Platform Detection for NAS Targets

**Priority:** Low–Medium

### smart-sniffer approach

`detectPlatform()` uses `os.Stat` checks against well-known marker files to identify the host platform at runtime with zero external commands:

| Check                          | Platform                |
| ------------------------------ | ----------------------- |
| `/etc/synoinfo.conf` exists    | Synology                |
| `/dev/sata1` exists            | Synology (older models) |
| `/etc/config/qpkg.conf` exists | QNAP                    |

On Synology, the standard `--scan` output may be empty because drives are exposed as `/dev/sata1`–`/dev/sata8` rather than `/dev/sd*`. smart-sniffer probes these paths directly when the standard scan returns zero devices.

### Current state in smartmontools-go

No platform detection. `ScanDevices` returns whatever `--scan-open` reports, which is an empty list on Synology without additional configuration.

### Recommendation

Add an internal `detectPlatform()` with the same `os.Stat`-based checks. Gate the Synology device path injection behind a `WithPlatformAutoDetect()` option (off by default) to avoid surprising behaviour on non-NAS deployments.

---

## Finding 7: Wear Level Normalization Across Drive Types

**Priority:** Low–Medium

### smart-sniffer approach

v0.5.6 introduced wear level unification: a single computed field derived from different SMART sources depending on the drive protocol:

| Drive type | Source                                                                                   |
| ---------- | ---------------------------------------------------------------------------------------- |
| ATA SSD    | Attributes 173 (SSD Life Used), 177 (Wear Leveling Count), 231 (SSD Life Left), 233, 234 |
| NVMe       | `nvme_smart_health_information_log.percentage_used`                                      |
| HDD        | Not applicable (returns nil/absent)                                                      |

### Current state in smartmontools-go

`SMARTInfo` exposes the full `AtaSmartData.Table` and `NvmeSmartHealth.PercentageUsed`. Computing a normalized wear level requires consumers to understand protocol-specific attribute IDs, which is a significant knowledge burden.

The constants `SmartAttrSSDLifeLeft = 231`, `SmartAttrSandForceInternal = 233`, and `SmartAttrTotalLBAsWritten = 234` already exist in [client.go](../client.go) for SSD detection logic.

### Recommendation

Add a `WearLevelPercent() *int` method on `SMARTInfo` that returns a normalized 0–100 value for SSDs and NVMe drives, or `nil` for HDDs and unknown types. This leverages the constants already present in the codebase and provides a meaningful convenience abstraction.

---

## Summary

| #   | Finding                                                | Priority   | Effort     | Status     |
| --- | ------------------------------------------------------ | ---------- | ---------- | ---------- |
| 1   | Multi-path smartctl binary fallback                    | High       | Low        | ✅ Resolved |
| 2   | SAT protocol auto-retry on execution failure bits      | High       | Low–Medium | ✅ Resolved |
| 3   | Full exit code bit decomposition (exec vs health bits) | Medium     | Medium     | ✅ Resolved |
| 4   | `--scan-open` → `--scan` fallback in `ScanDevices`     | Medium     | Low        | ✅ Resolved |
| 5   | Drive discovery / protocol probe mode                  | Medium     | Medium     | ✅ Resolved |
| 6   | Platform detection for Synology/QNAP                   | Low–Medium | Low        |            |
| 7   | Wear level normalization across drive types            | Low–Medium | Low        |            |

Findings 1, 2, and 4 are the highest return-on-effort items: they are small, self-contained changes to existing functions that directly improve reliability on the NAS and USB-attached devices that represent the primary real-world use case for a smartmontools Go library.
