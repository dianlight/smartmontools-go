package smartmontools

import (
	"context"
	"testing"
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
			"/usr/sbin/smartctl --nocheck=standby -a -j /dev/sda": {
				output: []byte(mockJSON),
			},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	info, err := client.GetSMARTInfo(context.Background(), "/dev/sda")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if info == nil {
		t.Fatal("Expected SMARTInfo to be returned")
	}

	// Verify device info is parsed correctly
	if info.Device.Name != "/dev/sda" {
		t.Errorf("Expected device name /dev/sda, got %s", info.Device.Name)
	}

	if info.ModelName != "Test Drive" {
		t.Errorf("Expected model name 'Test Drive', got %s", info.ModelName)
	}
}

// TestCheckHealthWithNoCheckStandby verifies --nocheck=standby is added
func TestCheckHealthWithNoCheckStandby(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl --nocheck=standby -H /dev/sda": {
				output: []byte("SMART overall-health self-assessment test result: PASSED"),
			},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	healthy, err := client.CheckHealth(context.Background(), "/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !healthy {
		t.Error("Expected device to be healthy")
	}
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
			"/usr/sbin/smartctl --nocheck=standby -i -j /dev/sda": {
				output: []byte(mockJSON),
			},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	info, err := client.GetDeviceInfo(context.Background(), "/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if model, ok := info["model_name"].(string); !ok || model != "Test Drive" {
		t.Errorf("Expected model_name 'Test Drive', got %v", info["model_name"])
	}
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
			"/usr/sbin/smartctl --nocheck=standby -c -j /dev/sda": {
				output: []byte(mockJSON),
			},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	info, err := client.GetAvailableSelfTests(context.Background(), "/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(info.Available) != 2 { // short and long
		t.Errorf("Expected 2 tests, got %d", len(info.Available))
	}
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
			if result != tt.expected {
				t.Errorf("isATADevice(%q) = %v, expected %v", tt.deviceType, result, tt.expected)
			}
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
			"/usr/sbin/smartctl -d sat --nocheck=standby -a -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	// Pre-cache the device type
	c := client.(*Client)
	c.setCachedDeviceType("/dev/sda", "sat")

	info, err := client.GetSMARTInfo(context.Background(), "/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if info.Device.Name != "/dev/sda" {
		t.Errorf("Expected device name /dev/sda, got %s", info.Device.Name)
	}

	if info.ModelName != "Test Drive" {
		t.Errorf("Expected model name 'Test Drive', got %s", info.ModelName)
	}
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
			"/usr/sbin/smartctl -d nvme -a -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client, _ := NewClient(WithSmartctlPath("/usr/sbin/smartctl"), WithCommander(commander))

	// Pre-cache the device type
	c := client.(*Client)
	c.setCachedDeviceType("/dev/nvme0n1", "nvme")

	info, err := client.GetSMARTInfo(context.Background(), "/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if info.Device.Name != "/dev/nvme0n1" {
		t.Errorf("Expected device name /dev/nvme0n1, got %s", info.Device.Name)
	}

	if info.DiskType != "NVMe" {
		t.Errorf("Expected disk type 'NVMe', got %s", info.DiskType)
	}
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

	if info.Device.Name != "/dev/sda" {
		t.Errorf("Expected device name /dev/sda, got %s", info.Device.Name)
	}
}
