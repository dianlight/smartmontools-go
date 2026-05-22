package compare

import smtypes "github.com/dianlight/smartmontools-go/internal/types"

// Shared interface aliases keep the compare backend decoupled from the root package.
type (
	// LogAdapter is an alias for smtypes.LogAdapter; it provides a log-handler interface.
	LogAdapter = smtypes.LogAdapter
	// Backend is an alias for smtypes.Backend; it is the pluggable SMART execution interface.
	Backend = smtypes.Backend
	// DiscoveryBackend is an alias for smtypes.DiscoveryBackend; it extends Backend with richer device discovery.
	DiscoveryBackend = smtypes.DiscoveryBackend
)

// Shared type aliases reuse the module's SMART domain model in the compare backend.
type (
	// Device is an alias for smtypes.Device representing a storage device.
	Device = smtypes.Device
	// SMARTInfo is an alias for smtypes.SMARTInfo holding comprehensive SMART data for a device.
	SMARTInfo = smtypes.SMARTInfo
	// SmartctlInfo is an alias for smtypes.SmartctlInfo holding smartctl metadata.
	SmartctlInfo = smtypes.SmartctlInfo
	// SelfTestInfo is an alias for smtypes.SelfTestInfo describing available self-tests.
	SelfTestInfo = smtypes.SelfTestInfo
	// DiscoveryResult is an alias for smtypes.DiscoveryResult holding the outcome of probing a device.
	DiscoveryResult = smtypes.DiscoveryResult
)
