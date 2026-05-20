// Package lib provides a Backend implementation that loads the smartmon wrapper
// library via purego (no CGO required). It is available on Linux and macOS.
//
// Build the wrapper library once with scripts/setup-lib-backend.sh, then use
// WithLibraryPath or SMARTMON_LIB_PATH to point the backend at it.
package lib

import smtypes "github.com/dianlight/smartmontools-go/internal/types"

// Shared interface aliases keep the lib backend decoupled from the root package.
type (
	LogAdapter = smtypes.LogAdapter
	Backend    = smtypes.Backend
)

// Shared type aliases reuse the module's SMART domain model in the lib backend.
type (
	Device        = smtypes.Device
	SMARTInfo     = smtypes.SMARTInfo
	SmartStatus   = smtypes.SmartStatus
	SmartSupport  = smtypes.SmartSupport
	AtaSmartData  = smtypes.AtaSmartData
	SelfTestInfo  = smtypes.SelfTestInfo
	DiscoveryResult = smtypes.DiscoveryResult
)
