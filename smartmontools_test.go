package smartmontools

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// mockCmd implements Cmd interface for testing
type mockCmd struct {
	output []byte
	err    error
}

func (m *mockCmd) Output() ([]byte, error) {
	return m.output, m.err
}

func (m *mockCmd) Run() error {
	return m.err
}

// mockCommander implements Commander interface for testing
type mockCommander struct {
	cmds map[string]*mockCmd
}

func (m *mockCommander) Command(name string, arg ...string) Cmd {
	key := name
	for _, a := range arg {
		key += " " + a
	}
	if cmd, ok := m.cmds[key]; ok {
		return cmd
	}
	// Default mock command that returns error
	return &mockCmd{err: errors.New("mock command not configured")}
}

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skip("smartctl not found in PATH, skipping test")
	}

	c := client.(*Client)
	if c.smartctlPath == "" {
		t.Error("Expected smartctlPath to be set")
	}
}

func TestNewClientWithPath(t *testing.T) {
	testPath := "/usr/sbin/smartctl"
	client := NewClientWithPath(testPath)

	c := client.(*Client)
	if c.smartctlPath != testPath {
		t.Errorf("Expected smartctlPath to be %s, got %s", testPath, c.smartctlPath)
	}
}

func TestScanDevices(t *testing.T) {
	mockJSON := `{
		"devices": [
			{"name": "/dev/sda", "type": "ata"},
			{"name": "/dev/sdb", "type": "ata"}
		]
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl --scan-open --json": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	devices, err := client.ScanDevices()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}

	if devices[0].Name != "/dev/sda" || devices[0].Type != "ata" {
		t.Errorf("Unexpected device 0: %+v", devices[0])
	}
}

func TestScanDevicesError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl --scan-open --json": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	_, err := client.ScanDevices()
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestGetSMARTInfo(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"model_name": "Test Drive",
		"serial_number": "12345",
		"smart_status": {"passed": true},
		"smartctl": {
			"messages": [
				{"string": "Test informational message", "severity": "info"}
			]
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetSMARTInfo("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if info.Device.Name != "/dev/sda" {
		t.Errorf("Expected device name /dev/sda, got %s", info.Device.Name)
	}

	if info.ModelName != "Test Drive" {
		t.Errorf("Expected model Test Drive, got %s", info.ModelName)
	}

	if !info.SmartStatus.Passed {
		t.Error("Expected SMART status passed")
	}

	if info.Smartctl == nil || len(info.Smartctl.Messages) != 1 {
		t.Errorf("Expected 1 message, got %v", info.Smartctl)
	}

	if info.Smartctl.Messages[0].String != "Test informational message" {
		t.Errorf("Expected message 'Test informational message', got '%s'", info.Smartctl.Messages[0].String)
	}

	if info.Smartctl.Messages[0].Severity != "info" {
		t.Errorf("Expected severity 'info', got '%s'", info.Smartctl.Messages[0].Severity)
	}
}

func TestGetSMARTInfoExitError(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"smart_status": {"passed": false},
		"smartctl": {
			"messages": [
				{"string": "Test error message", "severity": "error"}
			]
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {
				output: []byte(mockJSON),
				err:    &exec.ExitError{Stderr: []byte("")},
			},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetSMARTInfo("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if info.SmartStatus.Passed {
		t.Error("Expected SMART status failed")
	}

	if info.Smartctl == nil || len(info.Smartctl.Messages) != 1 {
		t.Errorf("Expected 1 message, got %v", info.Smartctl)
	}

	if info.Smartctl.Messages[0].String != "Test error message" {
		t.Errorf("Expected message 'Test error message', got '%s'", info.Smartctl.Messages[0].String)
	}

	if info.Smartctl.Messages[0].Severity != "error" {
		t.Errorf("Expected severity 'error', got '%s'", info.Smartctl.Messages[0].Severity)
	}
}

func TestCheckHealth(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -H /dev/sda": {output: []byte("SMART overall-health self-assessment test result: PASSED")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	healthy, err := client.CheckHealth("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !healthy {
		t.Error("Expected device to be healthy")
	}
}

func TestCheckHealthFailed(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -H /dev/sda": {output: []byte("SMART overall-health self-assessment test result: FAILED")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	healthy, err := client.CheckHealth("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if healthy {
		t.Error("Expected device to be unhealthy")
	}
}

func TestCheckHealthExitError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -H /dev/sda": {
				output: []byte("some output"),
				err:    &exec.ExitError{Stderr: []byte("SMART overall-health self-assessment test result: PASSED")},
			},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	healthy, err := client.CheckHealth("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !healthy {
		t.Error("Expected device to be healthy from stderr")
	}
}

func TestGetDeviceInfo(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"model_name": "Test Drive",
		"serial_number": "12345"
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -i -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetDeviceInfo("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if model, ok := info["model_name"].(string); !ok || model != "Test Drive" {
		t.Errorf("Expected model_name 'Test Drive', got %v", info["model_name"])
	}
}

func TestRunSelfTest(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -t short /dev/sda": {},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.RunSelfTest("/dev/sda", "short")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestRunSelfTestInvalidType(t *testing.T) {
	client := NewClientWithPath("/usr/sbin/smartctl")

	err := client.RunSelfTest("/dev/sda", "invalid")
	if err == nil {
		t.Error("Expected error for invalid test type")
	}
}

func TestRunSelfTestWithProgressInvalidType(t *testing.T) {
	client := NewClientWithPath("/usr/sbin/smartctl")

	ctx := context.Background()
	err := client.RunSelfTestWithProgress(ctx, "/dev/sda", "invalid", nil)
	if err == nil {
		t.Error("Expected error for invalid test type")
	}
}

func TestRunSelfTestWithProgress(t *testing.T) {
	// Mock SMART info with ATA device supporting self-tests and completed status
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"ata_smart_data": {
			"capabilities": {
				"exec_offline_immediate_supported": true
			},
			"self_test": {
				"status": "completed",
				"polling_minutes": {
					"short": 2
				}
			}
		}
	}`

	mockCapabilitiesJSON := `{
		"ata_smart_data": {
			"capabilities": {
				"exec_offline_immediate_supported": true,
				"self_tests_supported": true
			},
			"self_test": {
				"polling_minutes": {
					"short": 2
				}
			}
		}
	}`

	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda":    {output: []byte(mockJSON)},
			"/usr/sbin/smartctl -c -j /dev/sda":    {output: []byte(mockCapabilitiesJSON)},
			"/usr/sbin/smartctl -t short /dev/sda": {},
		},
	}

	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var progressCalled bool
	var finalProgress int
	var finalStatus string

	callback := func(progress int, status string) {
		progressCalled = true
		finalProgress = progress
		finalStatus = status
	}

	err := client.RunSelfTestWithProgress(ctx, "/dev/sda", "short", callback)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !progressCalled {
		t.Error("Expected progress callback to be called")
	}

	if finalProgress != 100 {
		t.Errorf("Expected final progress 100, got %d", finalProgress)
	}

	if finalStatus != "Self-test completed" {
		t.Errorf("Expected final status 'Self-test completed', got '%s'", finalStatus)
	}
}

func TestGetAvailableSelfTestsATA(t *testing.T) {
	mockJSON := `{
		"ata_smart_data": {
			"capabilities": {
				"exec_offline_immediate_supported": true,
				"self_tests_supported": true,
				"conveyance_self_test_supported": true
			}
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetAvailableSelfTests("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expected := []string{"short", "long", "conveyance", "offline"}
	if len(info.Available) != len(expected) {
		t.Errorf("Expected %d tests, got %d", len(expected), len(info.Available))
	}

	for i, test := range expected {
		if i >= len(info.Available) || info.Available[i] != test {
			t.Errorf("Expected test %s at position %d, got %v", test, i, info.Available)
		}
	}
}

func TestGetAvailableSelfTestsATANoCapabilities(t *testing.T) {
	mockJSON := `{
		"ata_smart_data": {}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetAvailableSelfTests("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(info.Available) != 0 {
		t.Errorf("Expected no tests, got %v", info.Available)
	}
}

func TestGetAvailableSelfTestsNVMe(t *testing.T) {
	mockJSON := `{
		"nvme_controller_capabilities": {
			"self_test": true
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetAvailableSelfTests("/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expected := []string{"short"}
	if len(info.Available) != len(expected) || info.Available[0] != expected[0] {
		t.Errorf("Expected %v, got %v", expected, info.Available)
	}
}

func TestGetAvailableSelfTestsNVMeNoSupport(t *testing.T) {
	mockJSON := `{
		"nvme_controller_capabilities": {
			"self_test": false
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetAvailableSelfTests("/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(info.Available) != 0 {
		t.Errorf("Expected no tests, got %v", info.Available)
	}
}

func TestGetAvailableSelfTestsError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	_, err := client.GetAvailableSelfTests("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestIsSMARTSupportedATA(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"ata_smart_data": {
			"capabilities": {
				"exec_offline_immediate_supported": true
			}
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	supportInfo, err := client.IsSMARTSupported("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !supportInfo.Supported {
		t.Error("Expected SMART to be supported for ATA device")
	}

	if !supportInfo.Enabled {
		t.Error("Expected SMART to be enabled for ATA device")
	}
}

func TestIsSMARTSupportedNVMe(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/nvme0n1", "type": "nvme"},
		"nvme_smart_health_information_log": {
			"temperature": 35
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	supportInfo, err := client.IsSMARTSupported("/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !supportInfo.Supported {
		t.Error("Expected SMART to be supported for NVMe device")
	}

	if !supportInfo.Enabled {
		t.Error("Expected SMART to be enabled for NVMe device")
	}
}

func TestIsSMARTSupportedNVMeWithSmartSupport(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/nvme0n1", "type": "nvme"},
		"smart_support": {
			"available": true,
			"enabled": false
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	supportInfo, err := client.IsSMARTSupported("/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !supportInfo.Supported {
		t.Error("Expected SMART to be supported for NVMe device")
	}

	if supportInfo.Enabled {
		t.Error("Expected SMART to be disabled for NVMe device")
	}
}

func TestIsSMARTSupportedNotSupported(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	supportInfo, err := client.IsSMARTSupported("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if supportInfo.Supported {
		t.Error("Expected SMART to not be supported")
	}

	if supportInfo.Enabled {
		t.Error("Expected SMART to not be enabled")
	}
}

func TestIsSMARTSupportedError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	_, err := client.IsSMARTSupported("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestEnableSMART(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -s on /dev/sda": {},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.EnableSMART("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestEnableSMARTError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -s on /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.EnableSMART("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestDisableSMART(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -s off /dev/sda": {},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.DisableSMART("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestDisableSMARTError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -s off /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.DisableSMART("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestAbortSelfTest(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -X /dev/sda": {},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.AbortSelfTest("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestAbortSelfTestError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -X /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.AbortSelfTest("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}
