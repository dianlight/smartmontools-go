/*
Package smartmontools provides Go bindings for interacting with smartmontools
and collecting S.M.A.R.T. data from storage devices.

The root package is a thin facade over the shared domain model in internal/types
and the default exec backend in backends/exec. NewClient creates a Client that
delegates SMART operations to a pluggable Backend implementation. By default it
uses ExecBackend, which shells out to the smartctl binary. Alternative backends
can be supplied with WithBackend.

# Features

  - Device scanning and discovery
  - SMART health status checking
  - Detailed SMART attribute reading
  - Disk type detection (SSD, HDD, NVMe, Unknown)
  - Rotation rate (RPM) information for HDDs
  - Temperature monitoring
  - Power-on time tracking
  - Self-test execution and progress monitoring
  - Device information retrieval
  - SMART support detection and management
  - Self-test availability checking
  - Standby mode detection for ATA-family devices
  - Efficient SMART monitoring with minimal disk I/O

# Backend Layout

The default smartctl-backed implementation lives in
github.com/dianlight/smartmontools-go/backends/exec. Shared types and
interfaces are hosted in an internal package to avoid circular imports while
keeping the public API backward compatible through type aliases in the root
package.
*/
package smartmontools
