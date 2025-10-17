package smartmontools

import (
	"errors"
	"os/exec"
	"testing"
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
		"smart_status": {"passed": true}
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
}

func TestGetSMARTInfoExitError(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"smart_status": {"passed": false}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {
				output: []byte("some output"),
				err:    &exec.ExitError{Stderr: []byte(mockJSON)},
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
