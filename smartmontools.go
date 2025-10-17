// Package smartmontools provides Go bindings for interfacing with smartmontools
// using libgoffi to call native smartctl functionality.
package smartmontools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Commander interface for executing commands
type Commander interface {
	Command(name string, arg ...string) Cmd
}

// Cmd interface for command execution
type Cmd interface {
	Output() ([]byte, error)
	Run() error
}

// execCommander implements Commander using os/exec
type execCommander struct{}

func (e execCommander) Command(name string, arg ...string) Cmd {
	return exec.Command(name, arg...)
}

// Device represents a storage device
type Device struct {
	Name string
	Type string
}

// SMARTInfo represents SMART information for a device
type SMARTInfo struct {
	Device          Device        `json:"device"`
	ModelFamily     string        `json:"model_family,omitempty"`
	ModelName       string        `json:"model_name,omitempty"`
	SerialNumber    string        `json:"serial_number,omitempty"`
	Firmware        string        `json:"firmware_version,omitempty"`
	UserCapacity    int64         `json:"user_capacity,omitempty"`
	SmartStatus     SmartStatus   `json:"smart_status,omitempty"`
	AtaSmartData    *AtaSmartData `json:"ata_smart_data,omitempty"`
	Temperature     *Temperature  `json:"temperature,omitempty"`
	PowerOnTime     *PowerOnTime  `json:"power_on_time,omitempty"`
	PowerCycleCount int           `json:"power_cycle_count,omitempty"`
}

// SmartStatus represents the overall SMART health status
type SmartStatus struct {
	Passed bool `json:"passed"`
}

// AtaSmartData represents ATA SMART attributes
type AtaSmartData struct {
	OfflineDataCollection *OfflineDataCollection `json:"offline_data_collection,omitempty"`
	SelfTest              *SelfTest              `json:"self_test,omitempty"`
	Capabilities          *Capabilities          `json:"capabilities,omitempty"`
	Table                 []SmartAttribute       `json:"table,omitempty"`
}

// OfflineDataCollection represents offline data collection status
type OfflineDataCollection struct {
	Status            string `json:"status,omitempty"`
	CompletionSeconds int    `json:"completion_seconds,omitempty"`
}

// SelfTest represents self-test information
type SelfTest struct {
	Status         string `json:"status,omitempty"`
	PollingMinutes int    `json:"polling_minutes,omitempty"`
}

// Capabilities represents SMART capabilities
type Capabilities struct {
	Values               []int `json:"values,omitempty"`
	ExecOfflineImmediate bool  `json:"exec_offline_immediate_supported,omitempty"`
}

// SmartAttribute represents a single SMART attribute
type SmartAttribute struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Value      int    `json:"value"`
	Worst      int    `json:"worst"`
	Thresh     int    `json:"thresh"`
	WhenFailed string `json:"when_failed,omitempty"`
	Flags      Flags  `json:"flags"`
	Raw        Raw    `json:"raw"`
}

// Flags represents attribute flags
type Flags struct {
	Value         int    `json:"value"`
	String        string `json:"string"`
	PreFailure    bool   `json:"prefailure"`
	UpdatedOnline bool   `json:"updated_online"`
	Performance   bool   `json:"performance"`
	ErrorRate     bool   `json:"error_rate"`
	EventCount    bool   `json:"event_count"`
	AutoKeep      bool   `json:"auto_keep"`
}

// Raw represents raw attribute value
type Raw struct {
	Value  int64  `json:"value"`
	String string `json:"string"`
}

// Temperature represents device temperature
type Temperature struct {
	Current int `json:"current"`
}

// PowerOnTime represents power on time
type PowerOnTime struct {
	Hours int `json:"hours"`
}

// Client represents a smartmontools client
type Client struct {
	smartctlPath string
	commander    Commander
}

// NewClient creates a new smartmontools client
func NewClient() (*Client, error) {
	// Try to find smartctl in PATH
	path, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("smartctl not found in PATH: %w", err)
	}

	return &Client{
		smartctlPath: path,
		commander:    execCommander{},
	}, nil
}

// NewClientWithPath creates a new smartmontools client with a specific smartctl path
func NewClientWithPath(smartctlPath string) *Client {
	return &Client{
		smartctlPath: smartctlPath,
		commander:    execCommander{},
	}
}

// NewClientWithCommander creates a new client with a custom commander (for testing)
func NewClientWithCommander(smartctlPath string, commander Commander) *Client {
	return &Client{
		smartctlPath: smartctlPath,
		commander:    commander,
	}
}

// ScanDevices scans for available storage devices
func (c *Client) ScanDevices() ([]Device, error) {
	cmd := c.commander.Command(c.smartctlPath, "--scan-open", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to scan devices: %w", err)
	}

	var result struct {
		Devices []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"devices"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse scan output: %w", err)
	}

	devices := make([]Device, len(result.Devices))
	for i, d := range result.Devices {
		devices[i] = Device{
			Name: d.Name,
			Type: d.Type,
		}
	}

	return devices, nil
}

// GetSMARTInfo retrieves SMART information for a device
func (c *Client) GetSMARTInfo(devicePath string) (*SMARTInfo, error) {
	cmd := c.commander.Command(c.smartctlPath, "-a", "-j", devicePath)
	output, err := cmd.Output()
	if err != nil {
		// smartctl returns non-zero exit codes for various conditions
		// We still want to parse the output if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			output = exitErr.Stderr
			if len(output) == 0 {
				return nil, fmt.Errorf("failed to get SMART info: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to execute smartctl: %w", err)
		}
	}

	var smartInfo SMARTInfo
	if err := json.Unmarshal(output, &smartInfo); err != nil {
		return nil, fmt.Errorf("failed to parse SMART info: %w", err)
	}

	return &smartInfo, nil
}

// CheckHealth checks if a device is healthy according to SMART
func (c *Client) CheckHealth(devicePath string) (bool, error) {
	cmd := c.commander.Command(c.smartctlPath, "-H", devicePath)
	output, err := cmd.Output()
	if err != nil {
		// Exit code 0: healthy, non-zero may indicate issues
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Parse output to determine health
			outputStr := string(exitErr.Stderr)
			if len(outputStr) == 0 {
				outputStr = string(output)
			}
			return strings.Contains(outputStr, "PASSED"), nil
		}
		return false, fmt.Errorf("failed to check health: %w", err)
	}

	outputStr := string(output)
	return strings.Contains(outputStr, "PASSED"), nil
}

// GetDeviceInfo retrieves basic device information
func (c *Client) GetDeviceInfo(devicePath string) (map[string]interface{}, error) {
	cmd := c.commander.Command(c.smartctlPath, "-i", "-j", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse device info: %w", err)
	}

	return info, nil
}

// RunSelfTest initiates a SMART self-test
func (c *Client) RunSelfTest(devicePath string, testType string) error {
	// Valid test types: short, long, conveyance, offline
	validTypes := map[string]bool{
		"short":      true,
		"long":       true,
		"conveyance": true,
		"offline":    true,
	}

	if !validTypes[testType] {
		return fmt.Errorf("invalid test type: %s (must be one of: short, long, conveyance, offline)", testType)
	}

	cmd := c.commander.Command(c.smartctlPath, "-t", testType, devicePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run self-test: %w", err)
	}

	return nil
}
