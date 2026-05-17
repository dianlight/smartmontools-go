// Package lib provides a Backend implementation that loads libsmartctl via
// purego (no CGO required). It is available on Linux, macOS, and FreeBSD.
// Use New to create a LibBackend that dlopen's a pre-built libsmartctl shared
// library and delegates all SMART operations through its C API.
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
