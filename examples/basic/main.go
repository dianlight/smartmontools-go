package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dianlight/smartmontools-go"
	"github.com/fatih/color"
)

func main() {
	// Create a new smartmontools client
	client, err := smartmontools.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	//slog.SetLogLoggerLevel(slog.LevelDebug)

	// Define color functions
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()

	fmt.Println(blue("Smartmontools Go Example"))
	fmt.Println(blue("========================="))
	fmt.Println()

	// Scan for available devices
	fmt.Println(blue("Scanning for devices..."))
	devices, err := client.ScanDevices()
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: Failed to scan devices: %v", err)))
		fmt.Println("Attempting to use /dev/sda as fallback...")
		devices = []smartmontools.Device{{Name: "/dev/sda", Type: "auto"}}
	}

	if len(devices) == 0 {
		fmt.Println(red("No devices found."))
		os.Exit(1)
	}

	fmt.Printf("Found %s device(s):\n", green(fmt.Sprintf("%d", len(devices))))
	for i, device := range devices {
		fmt.Printf("  %d. %s (type: %s)\n", i+1, device.Name, device.Type)
	}
	fmt.Println()

	// Use the first device for demonstration
	devicePath := devices[0].Name
	fmt.Printf("Using device: %s\n\n", blue(devicePath))

	// Check health status
	fmt.Println(blue("Checking device health..."))
	healthy, err := client.CheckHealth(devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: Failed to check health: %v", err)))
	} else {
		if healthy {
			fmt.Println(green("✓ Device health: PASSED"))
		} else {
			fmt.Println(red("✗ Device health: FAILED"))
		}
	}
	fmt.Println()
	// Check if SMART is supported
	fmt.Println(blue("Checking if SMART is supported..."))
	smartSupported, err := client.IsSMARTSupported(devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: Failed to check SMART support: %v", err)))
	} else {
		if smartSupported.Supported {
			fmt.Println(green("✓ SMART is supported"))
		} else {
			fmt.Println(red("✗ SMART is not supported"))
		}
		if smartSupported.Enabled {
			fmt.Println(green("✓ SMART is enabled"))
		} else {
			fmt.Println(red("✗ SMART is disabled"))
		}
	}
	fmt.Println()
	// Get basic device information
	fmt.Println(blue("Getting device information..."))
	info, err := client.GetDeviceInfo(devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: Failed to get device info: %v", err)))
	} else {
		fmt.Println(blue("Device Information:"))
		if modelName, ok := info["model_name"].(string); ok {
			fmt.Printf("  Model: %s\n", modelName)
		}
		if serialNumber, ok := info["serial_number"].(string); ok {
			fmt.Printf("  Serial: %s\n", serialNumber)
		}
		if firmware, ok := info["firmware_version"].(string); ok {
			fmt.Printf("  Firmware: %s\n", firmware)
		}
	}
	fmt.Println()

	// Get full SMART information
	fmt.Println(blue("Getting SMART information..."))
	smartInfo, err := client.GetSMARTInfo(devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: Failed to get SMART info: %v", err)))
	} else {
		fmt.Println(blue("SMART Information:"))
		fmt.Printf("  Model: %s\n", smartInfo.ModelName)
		fmt.Printf("  Serial: %s\n", smartInfo.SerialNumber)
		fmt.Printf("  Firmware: %s\n", smartInfo.Firmware)

		if smartInfo.Temperature != nil {
			fmt.Printf("  Temperature: %d°C\n", smartInfo.Temperature.Current)
		}

		if smartInfo.PowerOnTime != nil {
			fmt.Printf("  Power On Hours: %d\n", smartInfo.PowerOnTime.Hours)
		}

		fmt.Printf("  Power Cycle Count: %d\n", smartInfo.PowerCycleCount)

		if smartInfo.AtaSmartData != nil && len(smartInfo.AtaSmartData.Table) > 0 {
			fmt.Println("\n  Key SMART Attributes:")
			for _, attr := range smartInfo.AtaSmartData.Table {
				// Show some important attributes
				if attr.ID == 5 || attr.ID == 9 || attr.ID == 194 || attr.ID == 197 || attr.ID == 198 {
					fmt.Printf("    %d. %s: %d (worst: %d, thresh: %d)\n",
						attr.ID, attr.Name, attr.Value, attr.Worst, attr.Thresh)
				}
			}
		}
	}

	// Get available self-tests
	fmt.Println(blue("Getting available self-tests..."))
	availableTests, err := client.GetAvailableSelfTests(devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: Failed to get available tests: %v", err)))
	} else {
		fmt.Println(blue("Available Self-Tests:"))
		if len(availableTests) == 0 {
			fmt.Println("  None")
		} else {
			for _, test := range availableTests {
				fmt.Printf("  - %s\n", test)
			}
		}
	}
	fmt.Println()

	// Run short self-test with progress
	fmt.Println(blue("\nRunning short SMART self-test with progress..."))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	progressCallback := func(progress int, status string) {
		fmt.Printf("\rProgress: %d%% - %s", progress, status)
		if progress == 100 {
			fmt.Println() // New line after completion
		}
	}

	err = client.RunShortSelfTestWithProgress(ctx, devicePath, progressCallback)
	if err != nil {
		if strings.Contains(err.Error(), "not supported") {
			fmt.Println(yellow(fmt.Sprintf("\nNote: Self-tests are not supported by this device (%s)", devicePath)))
		} else {
			fmt.Println(yellow(fmt.Sprintf("Warning: Failed to run short self-test: %v", err)))
		}
	} else {
		fmt.Println(green("✓ Short self-test completed successfully"))
	}

	fmt.Println(green("\n✓ Example completed successfully"))
}
