package smartmontools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetSMARTInfoWithNoCheckStandby verifies --nocheck=standby is added for ATA devices
func TestGetSMARTInfoWithNoCheckStandby(t *testing.T) {
	mockJSON := `{
		"json_format_version": [1, 0],
		"smartctl": {
			"version": [7, 5],
			"exit_status": 0
		},
		"device": {
			"name": "/dev/sda",
			"type": "sat"
		},
		"model_name": "Test Drive",
		"smart_status": {"passed": true}
	}`

	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j --nocheck=standby /dev/sda": {
				output: []byte(mockJSON),
			},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	info, err := client.GetSMARTInfo(context.Background(), "/dev/sda")
	assert.NoError(t, err)
	assert.NotNil(t, info, "Expected SMARTInfo to be returned")

	// Verify device info is parsed correctly
	assert.Equal(t, "/dev/sda", info.Device.Name)
	assert.Equal(t, "Test Drive", info.ModelName)
}

// TestCheckHealthWithNoCheckStandby verifies --nocheck=standby is added
func TestCheckHealthWithNoCheckStandby(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -H --nocheck=standby /dev/sda": {
				output: []byte("SMART overall-health self-assessment test result: PASSED"),
			},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	healthy, err := client.CheckHealth(context.Background(), "/dev/sda")
	assert.NoError(t, err)
	assert.True(t, healthy, "Expected device to be healthy")
}

// TestGetDeviceInfoWithNoCheckStandby verifies --nocheck=standby is added
func TestGetDeviceInfoWithNoCheckStandby(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"model_name": "Test Drive",
		"serial_number": "12345"
	}`

	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -i -j --nocheck=standby /dev/sda": {
				output: []byte(mockJSON),
			},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	info, err := client.GetDeviceInfo(context.Background(), "/dev/sda")
	assert.NoError(t, err)
	model, ok := info["model_name"].(string)
	assert.True(t, ok, "Expected model_name to be a string")
	assert.Equal(t, "Test Drive", model)
}

// TestGetAvailableSelfTestsWithNoCheckStandby verifies --nocheck=standby is added
func TestGetAvailableSelfTestsWithNoCheckStandby(t *testing.T) {
	mockJSON := `{
		"ata_smart_data": {
			"capabilities": {
				"self_tests_supported": true
			}
		}
	}`

	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j --nocheck=standby /dev/sda": {
				output: []byte(mockJSON),
			},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	info, err := client.GetAvailableSelfTests(context.Background(), "/dev/sda")
	assert.NoError(t, err)
	assert.Len(t, info.Available, 2, "Expected 2 tests (short and long)")
}

// TestIsATADevice tests the isATADevice helper function
func TestIsATADevice(t *testing.T) {
	tests := []struct {
		name       string
		deviceType string
		expected   bool
	}{
		{"ATA device", "ata", true},
		{"SAT device", "sat", true},
		{"SATA device", "sata", true},
		{"SCSI device", "scsi", true},
		{"Uppercase ATA", "ATA", true},
		{"Uppercase SAT", "SAT", true},
		{"NVMe device", "nvme", false},
		{"Empty string", "", false},
		{"Unknown type", "usb", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isATADevice(tt.deviceType)
			assert.Equal(t, tt.expected, result, "isATADevice(%q)", tt.deviceType)
		})
	}
}

// TestGetSMARTInfoWithCachedDeviceType tests that cached device types are used correctly
func TestGetSMARTInfoWithCachedATADeviceType(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "sat"},
		"model_name": "Test Drive",
		"smart_status": {"passed": true}
	}`

	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j --nocheck=standby -d sat /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	// Pre-cache the device type
	c := client.(*Client)
	c.setCachedDeviceType("/dev/sda", "sat")

	info, err := client.GetSMARTInfo(context.Background(), "/dev/sda")
	assert.NoError(t, err)
	assert.Equal(t, "/dev/sda", info.Device.Name)
	assert.Equal(t, "Test Drive", info.ModelName)
}

// TestGetSMARTInfoWithCachedNVMeDeviceType tests NVMe devices don't get --nocheck=standby
func TestGetSMARTInfoWithCachedNVMeDeviceType(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/nvme0n1", "type": "nvme"},
		"model_name": "NVMe Drive",
		"nvme_smart_health_information_log": {"temperature": 35}
	}`

	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j -d nvme /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	// Pre-cache the device type
	c := client.(*Client)
	c.setCachedDeviceType("/dev/nvme0n1", "nvme")

	info, err := client.GetSMARTInfo(context.Background(), "/dev/nvme0n1")
	assert.NoError(t, err)
	assert.Equal(t, "/dev/nvme0n1", info.Device.Name)
	assert.Equal(t, "NVMe", info.DiskType)
}

// TestInStandbyField tests that the InStandby field is properly exposed
func TestInStandbyField(t *testing.T) {
	info := SMARTInfo{
		Device:    Device{Name: "/dev/sda", Type: "sat"},
		InStandby: true,
	}

	if !info.InStandby {
		t.Error("Expected InStandby to be true")
	}

	assert.True(t, info.InStandby, "Expected InStandby to be true")
	assert.Equal(t, "/dev/sda", info.Device.Name)
}
