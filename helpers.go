package smartmontools

import "strings"

// isATADevice checks if a device type is ATA-based (ata, sat, sata, etc.)
func isATADevice(deviceType string) bool {
	if deviceType == "" {
		return false
	}
	dt := strings.ToLower(deviceType)
	return strings.Contains(dt, "ata") || strings.Contains(dt, "sat") || dt == "scsi"
}

// determineDiskType determines the type of disk based on available information.
// Optimized to check conditions in order of likelihood and cost.
func determineDiskType(info *SMARTInfo) string {
	// Check for NVMe devices first
	if info.Device.Type == "nvme" || info.NvmeSmartHealth != nil || info.NvmeControllerCapabilities != nil {
		return "NVMe"
	}

	// Check rotation rate for ATA/SATA devices (most reliable indicator)
	if info.RotationRate != nil {
		if *info.RotationRate == 0 {
			return "SSD"
		}
		return "HDD"
	}

	// Check device type from smartctl
	deviceType := strings.ToLower(info.Device.Type)
	if strings.Contains(deviceType, "nvme") {
		return "NVMe"
	}

	if strings.Contains(deviceType, "sata") || strings.Contains(deviceType, "ata") || strings.Contains(deviceType, "sat") {
		// If we have ATA SMART data but no rotation rate, try to infer
		if info.AtaSmartData != nil {
			// Look for SSD-specific attributes
			for _, attr := range info.AtaSmartData.Table {
				if attr.ID == SmartAttrSSDLifeLeft || attr.ID == SmartAttrSandForceInternal || attr.ID == SmartAttrTotalLBAsWritten {
					return "SSD"
				}
			}
		}
	}

	// If we can't determine, return Unknown
	return "Unknown"
}
