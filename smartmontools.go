// Package smartmontools provides Go bindings for interfacing with smartmontools
// using libgoffi to call native smartctl functionality.
package smartmontools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
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
	slog.Debug("Executing command", "name", name, "args", arg)
	return exec.Command(name, arg...)
}

// Device represents a storage device
type Device struct {
	Name string
	Type string
}

// NvmeControllerCapabilities represents NVMe controller capabilities
type NvmeControllerCapabilities struct {
	SelfTest bool `json:"self_test,omitempty"`
}

// NvmeSmartHealth represents NVMe SMART health information
type NvmeSmartHealth struct {
	CriticalWarning      int   `json:"critical_warning,omitempty"`
	Temperature          int   `json:"temperature,omitempty"`
	AvailableSpare       int   `json:"available_spare,omitempty"`
	AvailableSpareThresh int   `json:"available_spare_threshold,omitempty"`
	PercentageUsed       int   `json:"percentage_used,omitempty"`
	DataUnitsRead        int64 `json:"data_units_read,omitempty"`
	DataUnitsWritten     int64 `json:"data_units_written,omitempty"`
	HostReadCommands     int64 `json:"host_read_commands,omitempty"`
	HostWriteCommands    int64 `json:"host_write_commands,omitempty"`
	ControllerBusyTime   int64 `json:"controller_busy_time,omitempty"`
	PowerCycles          int64 `json:"power_cycles,omitempty"`
	PowerOnHours         int64 `json:"power_on_hours,omitempty"`
	UnsafeShutdowns      int64 `json:"unsafe_shutdowns,omitempty"`
	MediaErrors          int64 `json:"media_errors,omitempty"`
	NumErrLogEntries     int64 `json:"num_err_log_entries,omitempty"`
	WarningTempTime      int   `json:"warning_temp_time,omitempty"`
	CriticalCompTime     int   `json:"critical_comp_time,omitempty"`
	TemperatureSensors   []int `json:"temperature_sensors,omitempty"`
}

// SMARTInfo represents SMART information for a device
type SMARTInfo struct {
	Device                     Device                      `json:"device"`
	ModelFamily                string                      `json:"model_family,omitempty"`
	ModelName                  string                      `json:"model_name,omitempty"`
	SerialNumber               string                      `json:"serial_number,omitempty"`
	Firmware                   string                      `json:"firmware_version,omitempty"`
	UserCapacity               int64                       `json:"user_capacity,omitempty"`
	SmartStatus                SmartStatus                 `json:"smart_status,omitempty"`
	SmartSupport               *SmartSupport               `json:"smart_support,omitempty"`
	AtaSmartData               *AtaSmartData               `json:"ata_smart_data,omitempty"`
	NvmeSmartHealth            *NvmeSmartHealth            `json:"nvme_smart_health_information_log,omitempty"`
	NvmeControllerCapabilities *NvmeControllerCapabilities `json:"nvme_controller_capabilities,omitempty"`
	Temperature                *Temperature                `json:"temperature,omitempty"`
	PowerOnTime                *PowerOnTime                `json:"power_on_time,omitempty"`
	PowerCycleCount            int                         `json:"power_cycle_count,omitempty"`
}

// SmartStatus represents the overall SMART health status
type SmartStatus struct {
	Passed bool `json:"passed"`
}

// SmartSupport represents SMART availability and enablement status
type SmartSupport struct {
	Available bool `json:"available"`
	Enabled   bool `json:"enabled"`
}

// SMARTSupportInfo represents SMART support and enablement information
type SMARTSupportInfo struct {
	Supported bool
	Enabled   bool
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
		// We still want to parse the output if available and it's valid JSON
		if len(output) > 0 {
			var smartInfo SMARTInfo
			if json.Unmarshal(output, &smartInfo) == nil {
				// Valid JSON, treat error as warning
				slog.Warn("smartctl returned error but provided valid JSON output", "error", err)
				return &smartInfo, nil
			}
		}
		return nil, fmt.Errorf("failed to get SMART info: %w", err)
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

// ProgressCallback is a function type for reporting progress
type ProgressCallback func(progress int, status string)

// RunShortSelfTestWithProgress starts a short SMART self-test and reports progress
func (c *Client) RunShortSelfTestWithProgress(ctx context.Context, devicePath string, callback ProgressCallback) error {
	// First check if self-tests are supported
	smartInfo, err := c.GetSMARTInfo(devicePath)
	if err != nil {
		return fmt.Errorf("failed to get SMART info: %w", err)
	}

	// Check ATA self-test support
	supportsSelfTest := false
	if smartInfo.AtaSmartData != nil && smartInfo.AtaSmartData.Capabilities != nil {
		supportsSelfTest = smartInfo.AtaSmartData.Capabilities.ExecOfflineImmediate
	}

	// Check NVMe self-test support
	if smartInfo.NvmeControllerCapabilities != nil {
		supportsSelfTest = smartInfo.NvmeControllerCapabilities.SelfTest
	}

	if !supportsSelfTest {
		return fmt.Errorf("self-tests are not supported by this device")
	}

	// Start the short self-test
	if err := c.RunSelfTest(devicePath, "short"); err != nil {
		return fmt.Errorf("failed to start short self-test: %w", err)
	}

	if callback != nil {
		callback(0, "Short self-test started")
	}

	// Get initial SMART info to determine expected duration
	expectedMinutes := 2 // default for short test
	if smartInfo.AtaSmartData != nil && smartInfo.AtaSmartData.SelfTest != nil {
		if smartInfo.AtaSmartData.SelfTest.PollingMinutes > 0 {
			expectedMinutes = smartInfo.AtaSmartData.SelfTest.PollingMinutes
		}
	}

	// Poll for completion
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	elapsed := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			elapsed += 5

			// Check current status
			currentInfo, err := c.GetSMARTInfo(devicePath)
			if err != nil {
				slog.Warn("Failed to get SMART info during polling", "error", err)
				continue
			}

			if currentInfo.AtaSmartData != nil && currentInfo.AtaSmartData.SelfTest != nil {
				status := currentInfo.AtaSmartData.SelfTest.Status
				if status == "completed" || status == "aborted" || status == "interrupted" {
					if callback != nil {
						callback(100, fmt.Sprintf("Self-test %s", status))
					}
					return nil
				}

				// Calculate progress based on elapsed time vs expected duration
				progress := (elapsed * 100) / (expectedMinutes * 60)
				if progress > 95 {
					progress = 95 // Don't show 100% until actually completed
				}

				if callback != nil {
					callback(progress, fmt.Sprintf("Self-test in progress (%s)", status))
				}
			} else {
				// Fallback progress calculation
				progress := (elapsed * 100) / (expectedMinutes * 60)
				if progress > 95 {
					progress = 95
				}
				if callback != nil {
					callback(progress, "Self-test in progress")
				}
			}

			// Timeout after 2x expected duration
			if elapsed > expectedMinutes*120 {
				return fmt.Errorf("self-test timed out after %d seconds", elapsed)
			}
		}
	}
}

// GetAvailableSelfTests returns the list of available self-test types for a device
func (c *Client) GetAvailableSelfTests(devicePath string) ([]string, error) {
	smartInfo, err := c.GetSMARTInfo(devicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SMART info: %w", err)
	}

	var tests []string

	// Check ATA capabilities
	if smartInfo.AtaSmartData != nil && smartInfo.AtaSmartData.Capabilities != nil {
		caps := smartInfo.AtaSmartData.Capabilities
		if caps.ExecOfflineImmediate {
			tests = append(tests, "short", "long", "offline")
		}
		// Conveyance test support - check if capability value indicates support
		// For simplicity, assume conveyance is supported if offline immediate is
		if caps.ExecOfflineImmediate {
			tests = append(tests, "conveyance")
		}
	}

	// Check NVMe capabilities
	if smartInfo.NvmeControllerCapabilities != nil && smartInfo.NvmeControllerCapabilities.SelfTest {
		tests = append(tests, "short")
	}

	return tests, nil
}

// IsSMARTSupported checks if SMART is supported on a device and if it's enabled
func (c *Client) IsSMARTSupported(devicePath string) (*SMARTSupportInfo, error) {
	smartInfo, err := c.GetSMARTInfo(devicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SMART info: %w", err)
	}

	supportInfo := &SMARTSupportInfo{}

	// Check NVMe SMART support first
	if smartInfo.SmartSupport != nil {
		supportInfo.Supported = smartInfo.SmartSupport.Available
		supportInfo.Enabled = smartInfo.SmartSupport.Enabled
		return supportInfo, nil
	}

	// Check ATA SMART data presence for support
	if smartInfo.AtaSmartData != nil {
		supportInfo.Supported = true
		// For ATA devices, if SMART data is present, assume it's enabled
		// (ATA devices typically don't have a separate enabled/disabled status in JSON)
		supportInfo.Enabled = true
		return supportInfo, nil
	}

	// Check NVMe SMART health information as fallback
	if smartInfo.NvmeSmartHealth != nil {
		supportInfo.Supported = true
		supportInfo.Enabled = true
		return supportInfo, nil
	}

	// Not supported
	supportInfo.Supported = false
	supportInfo.Enabled = false
	return supportInfo, nil
}

// EnableSMART enables SMART monitoring on a device
func (c *Client) EnableSMART(devicePath string) error {
	cmd := c.commander.Command(c.smartctlPath, "-s", "on", devicePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable SMART: %w", err)
	}
	return nil
}

// DisableSMART disables SMART monitoring on a device
func (c *Client) DisableSMART(devicePath string) error {
	cmd := c.commander.Command(c.smartctlPath, "-s", "off", devicePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable SMART: %w", err)
	}
	return nil
}
