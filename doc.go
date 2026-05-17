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

An alternative purego FFI backend lives in
github.com/dianlight/smartmontools-go/backends/lib (LibBackend). It loads
a pre-built libsmartctl shared library at runtime using ebitengine/purego,
avoiding process-spawn overhead and the smartctl binary dependency.

# LibBackend (D1 — Patch-and-Build Pipeline)

LibBackend requires libsmartctl.so (Linux/FreeBSD) or libsmartctl.dylib
(macOS) to be installed on the target system. The library is built from
the smartmontools source tree by applying the versioned patch set in the
patches/ directory:

	# Clone and patch smartmontools, then build the shared library:
	git clone --depth 1 --branch v7.5 \
	  https://github.com/smartmontools/smartmontools.git src
	./patches/apply.sh v7.5 src
	cd src && ./autogen.sh
	./configure --enable-shared --disable-static --enable-libsmartctl \
	  CFLAGS="-fPIC" CXXFLAGS="-fPIC -DBUILDING_LIBSMARTCTL"
	make -j$(nproc)
	sudo cp src/.libs/libsmartctl.so* /usr/local/lib/
	sudo ldconfig

Use the LibBackend with WithBackend:

	lib, err := libbackend.New(libbackend.WithLibraryPath("/usr/local/lib/libsmartctl.so"))
	if err != nil {
	    log.Fatal(err)
	}
	client, err := smartmontools.NewClient(smartmontools.WithBackend(lib))

Pre-built binaries for Linux (amd64, arm64) and macOS (amd64, arm64) are
published as GitHub Release assets by the build-libsmartctl workflow
(.github/workflows/build-libsmartctl.yml), which runs weekly against the
latest upstream smartmontools release.
*/
package smartmontools
