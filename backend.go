package smartmontools

import "context"

// Backend is the pluggable execution interface for SMART operations.
// The default implementation is ExecBackend (wraps the smartctl binary).
// Alternative implementations may use native ioctl calls, CGO bindings, or
// other mechanisms.
//
// All methods accept a [context.Context]. Implementations should use
// context.Background() as fallback when a nil context is received.
type Backend interface {
	// Name returns a human-readable identifier for this backend (e.g., "exec").
	Name() string

	// ScanDevices scans for available storage devices.
	ScanDevices(ctx context.Context) ([]Device, error)

	// GetSMARTInfo retrieves complete SMART information for a device.
	GetSMARTInfo(ctx context.Context, devicePath string) (*SMARTInfo, error)

	// CheckHealth checks if a device is healthy according to SMART.
	CheckHealth(ctx context.Context, devicePath string) (bool, error)

	// GetDeviceInfo retrieves basic device information.
	GetDeviceInfo(ctx context.Context, devicePath string) (map[string]interface{}, error)

	// RunSelfTest initiates a SMART self-test on a device.
	RunSelfTest(ctx context.Context, devicePath string, testType string) error

	// GetAvailableSelfTests returns the available self-test types and their durations.
	GetAvailableSelfTests(ctx context.Context, devicePath string) (*SelfTestInfo, error)

	// EnableSMART enables SMART monitoring on a device.
	EnableSMART(ctx context.Context, devicePath string) error

	// DisableSMART disables SMART monitoring on a device.
	// NVMe devices do not support this operation and an error will be returned.
	DisableSMART(ctx context.Context, devicePath string) error

	// AbortSelfTest aborts a running self-test on a device.
	AbortSelfTest(ctx context.Context, devicePath string) error

	// Close releases any resources held by the backend.
	Close() error
}

// DiscoveryBackend is an optional extension of [Backend] that provides richer
// device discovery with per-device protocol-fallback details. Backends that
// implement this interface can surface whether SAT or other protocol fallbacks
// were required for individual devices during [SmartClient.DiscoverDevices].
//
// [Client.DiscoverDevices] performs a type assertion to this interface at call
// time: if the active Backend implements DiscoveryBackend, the richer
// implementation is used; otherwise a generic fallback with no SAT-fallback
// details is applied.
type DiscoveryBackend interface {
	Backend
	DiscoverDevices(ctx context.Context) ([]DiscoveryResult, error)
}
