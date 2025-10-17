package smartmontools

import (
	"os"
	"testing"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skip("smartctl not found in PATH, skipping test")
	}

	if client.smartctlPath == "" {
		t.Error("Expected smartctlPath to be set")
	}
}

func TestNewClientWithPath(t *testing.T) {
	testPath := "/usr/sbin/smartctl"
	client := NewClientWithPath(testPath)

	if client.smartctlPath != testPath {
		t.Errorf("Expected smartctlPath to be %s, got %s", testPath, client.smartctlPath)
	}
}

func TestScanDevices(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skip("smartctl not found in PATH, skipping test")
	}

	// Try to scan devices - this might fail if run without proper permissions
	_, err = client.ScanDevices()
	if err != nil {
		t.Logf("Device scan failed (may require root): %v", err)
	}
}

func TestGetSMARTInfo(t *testing.T) {
	// This test requires a valid device path
	// We'll skip it if not running as root or no device available
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges")
	}

	client, err := NewClient()
	if err != nil {
		t.Skip("smartctl not found in PATH, skipping test")
	}

	// Try to find a device first
	devices, err := client.ScanDevices()
	if err != nil || len(devices) == 0 {
		t.Skip("No devices found to test")
	}

	// Test with first device
	_, err = client.GetSMARTInfo(devices[0].Name)
	if err != nil {
		t.Errorf("Failed to get SMART info: %v", err)
	}
}

func TestCheckHealth(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges")
	}

	client, err := NewClient()
	if err != nil {
		t.Skip("smartctl not found in PATH, skipping test")
	}

	devices, err := client.ScanDevices()
	if err != nil || len(devices) == 0 {
		t.Skip("No devices found to test")
	}

	_, err = client.CheckHealth(devices[0].Name)
	if err != nil {
		t.Logf("Health check failed: %v", err)
	}
}

func TestRunSelfTest(t *testing.T) {
	client := NewClientWithPath("/usr/sbin/smartctl")

	// Test invalid test type
	err := client.RunSelfTest("/dev/sda", "invalid")
	if err == nil {
		t.Error("Expected error for invalid test type")
	}

	// We won't actually run a test as it requires root and a real device
	if os.Getuid() != 0 {
		t.Skip("Skipping actual self-test (requires root)")
	}
}
