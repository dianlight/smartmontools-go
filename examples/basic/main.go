package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dianlight/smartmontools-go"
)

func main() {
	// Create a new smartmontools client
	client, err := smartmontools.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	//slog.SetLogLoggerLevel(slog.LevelDebug)

	fmt.Println("Smartmontools Go Example")
	fmt.Println("=========================")
	fmt.Println()

	// Scan for available devices
	fmt.Println("Scanning for devices...")
	devices, err := client.ScanDevices()
	if err != nil {
		log.Printf("Warning: Failed to scan devices: %v\n", err)
		fmt.Println("Attempting to use /dev/sda as fallback...")
		devices = []smartmontools.Device{{Name: "/dev/sda", Type: "auto"}}
	}

	if len(devices) == 0 {
		fmt.Println("No devices found.")
		os.Exit(1)
	}

	fmt.Printf("Found %d device(s):\n", len(devices))
	for i, device := range devices {
		fmt.Printf("  %d. %s (type: %s)\n", i+1, device.Name, device.Type)
	}
	fmt.Println()

	// Use the first device for demonstration
	devicePath := devices[0].Name
	fmt.Printf("Using device: %s\n\n", devicePath)

	// Check health status
	fmt.Println("Checking device health...")
	healthy, err := client.CheckHealth(devicePath)
	if err != nil {
		log.Printf("Warning: Failed to check health: %v\n", err)
	} else {
		if healthy {
			fmt.Println("✓ Device health: PASSED")
		} else {
			fmt.Println("✗ Device health: FAILED")
		}
	}
	fmt.Println()

	// Get basic device information
	fmt.Println("Getting device information...")
	info, err := client.GetDeviceInfo(devicePath)
	if err != nil {
		log.Printf("Warning: Failed to get device info: %v\n", err)
	} else {
		fmt.Println("Device Information:")
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
	fmt.Println("Getting SMART information...")
	smartInfo, err := client.GetSMARTInfo(devicePath)
	if err != nil {
		log.Printf("Warning: Failed to get SMART info: %v\n", err)
	} else {
		fmt.Println("SMART Information:")
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

	// Run short self-test with progress
	fmt.Println("\nRunning short SMART self-test with progress...")
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
			fmt.Printf("\nNote: Self-tests are not supported by this device (%s)\n", devicePath)
		} else {
			log.Printf("Warning: Failed to run short self-test: %v\n", err)
		}
	} else {
		fmt.Println("✓ Short self-test completed successfully")
	}

	fmt.Println("\n✓ Example completed successfully")
}
