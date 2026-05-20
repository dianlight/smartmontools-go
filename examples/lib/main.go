//go:build linux || darwin

// Package main demonstrates using the LibBackend (SDK) that loads the smartmon
// wrapper library at runtime via purego — no CGO required.
//
// Build the wrapper library first:
//
//	scripts/setup-lib-backend.sh
//
// Run the example:
//
//	SMARTMON_LIB_PATH=backends/lib/sdk/libsmartmon_go.dylib go run .  # macOS
//	SMARTMON_LIB_PATH=backends/lib/sdk/libsmartmon_go.so    go run .  # Linux
//
// Or pass the path explicitly:
//
//	go run . -lib /path/to/libsmartmon_go.so
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dianlight/tlog"
	"github.com/fatih/color"

	smartmontools "github.com/dianlight/smartmontools-go"
	libbackend "github.com/dianlight/smartmontools-go/backends/lib"
)

func main() {
	blue := color.New(color.FgBlue).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Println(blue("Smartmontools LibBackend (SDK) Example"))
	fmt.Println(blue("======================================"))
	fmt.Println()

	// Build options. An explicit library path can be provided via
	// SMARTMON_LIB_PATH; otherwise the backend searches well-known locations.
	libOpts := []libbackend.Option{
		libbackend.WithTLogHandler(tlog.NewLoggerWithLevel(tlog.LevelInfo)),
	}
	if path, ok := os.LookupEnv("SMARTMON_LIB_PATH"); ok {
		libOpts = append(libOpts, libbackend.WithLibraryPath(path))
		fmt.Printf("Using library: %s\n\n", blue(path))
	} else {
		fmt.Println(yellow("SMARTMON_LIB_PATH not set – searching default locations"))
		fmt.Println()
	}

	// Create the LibBackend. This dlopen()s the shared library and initialises
	// the smartmontools singleton; no child process is spawned.
	lib, err := libbackend.New(libOpts...)
	if err != nil {
		fmt.Println(red(fmt.Sprintf("✗ Failed to load smartmon wrapper: %v", err)))
		fmt.Println()
		fmt.Println("Build the wrapper library with:")
		fmt.Println("  scripts/setup-lib-backend.sh")
		fmt.Println("Then set SMARTMON_LIB_PATH to the resulting .so/.dylib path.")
		os.Exit(1)
	}
	defer func() {
		if err := lib.Close(); err != nil {
			tlog.Warn("Failed to close lib backend", "error", err)
		}
	}()

	fmt.Println(green("✓ Wrapper library loaded successfully"))
	fmt.Println()

	// Wire the LibBackend into the high-level smartmontools client.
	client, err := smartmontools.NewClient(smartmontools.WithBackend(lib))
	if err != nil {
		tlog.Fatal("Failed to create client", "error", err)
	}

	ctx := context.Background()

	// ── Device discovery ──────────────────────────────────────────────────
	fmt.Println(blue("Scanning for devices..."))
	devices, err := client.ScanDevices(ctx)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: %v", err)))
		devices = []smartmontools.Device{{Name: "/dev/sda", Type: "auto"}}
		fmt.Println("Falling back to /dev/sda")
	}
	if len(devices) == 0 {
		fmt.Println(red("No devices found. Ensure you have sufficient permissions (e.g. sudo)."))
		os.Exit(1)
	}

	fmt.Printf("Found %s device(s):\n", green(fmt.Sprintf("%d", len(devices))))
	for i, d := range devices {
		fmt.Printf("  %d. %s (type: %s)\n", i+1, d.Name, d.Type)
	}
	fmt.Println()

	// Use the first device for the rest of the demo.
	devicePath := devices[0].Name
	fmt.Printf("Using device: %s\n\n", blue(devicePath))

	// ── Health check ──────────────────────────────────────────────────────
	fmt.Println(blue("Checking device health..."))
	healthy, err := client.CheckHealth(ctx, devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: %v", err)))
	} else if healthy {
		fmt.Println(green("✓ Health: PASSED"))
	} else {
		fmt.Println(red("✗ Health: FAILED"))
	}
	fmt.Println()

	// ── SMART support ─────────────────────────────────────────────────────
	fmt.Println(blue("Checking SMART support..."))
	support, err := client.IsSMARTSupported(ctx, devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: %v", err)))
	} else {
		if support.Available {
			fmt.Println(green("✓ SMART available"))
		} else {
			fmt.Println(red("✗ SMART not available"))
		}
		if support.Enabled {
			fmt.Println(green("✓ SMART enabled"))
		} else {
			fmt.Println(red("✗ SMART disabled"))
		}
	}
	fmt.Println()

	// ── Device information ────────────────────────────────────────────────
	fmt.Println(blue("Getting device information..."))
	devInfo, err := client.GetDeviceInfo(ctx, devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: %v", err)))
	} else {
		if v, ok := devInfo["model_name"].(string); ok {
			fmt.Printf("  Model:    %s\n", v)
		}
		if v, ok := devInfo["serial_number"].(string); ok {
			fmt.Printf("  Serial:   %s\n", v)
		}
		if v, ok := devInfo["firmware_version"].(string); ok {
			fmt.Printf("  Firmware: %s\n", v)
		}
	}
	fmt.Println()

	// ── Full SMART data ───────────────────────────────────────────────────
	fmt.Println(blue("Getting SMART information..."))
	smartInfo, err := client.GetSMARTInfo(ctx, devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: %v", err)))
	} else {
		fmt.Printf("  Model:      %s\n", smartInfo.ModelName)
		fmt.Printf("  Serial:     %s\n", smartInfo.SerialNumber)
		fmt.Printf("  Firmware:   %s\n", smartInfo.Firmware)
		if smartInfo.DiskType != "" {
			fmt.Printf("  Disk Type:  %s\n", smartInfo.DiskType)
		}
		if smartInfo.RotationRate != nil {
			if *smartInfo.RotationRate > 0 {
				fmt.Printf("  RPM:        %d\n", *smartInfo.RotationRate)
			} else {
				fmt.Println("  RPM:        0 (non-rotating)")
			}
		}
		if smartInfo.Temperature != nil {
			fmt.Printf("  Temp:       %d°C\n", smartInfo.Temperature.Current)
		}
		if smartInfo.PowerOnTime != nil {
			fmt.Printf("  Power-on:   %d hours\n", smartInfo.PowerOnTime.Hours)
		}
		if smartInfo.AtaSmartData != nil && len(smartInfo.AtaSmartData.Table) > 0 {
			fmt.Println("\n  Key SMART Attributes (ID 5, 9, 194, 197, 198):")
			for _, attr := range smartInfo.AtaSmartData.Table {
				if attr.ID == 5 || attr.ID == 9 || attr.ID == 194 || attr.ID == 197 || attr.ID == 198 {
					fmt.Printf("    %3d %-30s value=%d worst=%d thresh=%d\n",
						attr.ID, attr.Name, attr.Value, attr.Worst, attr.Thresh)
				}
			}
		}
	}
	fmt.Println()

	// ── Available self-tests ──────────────────────────────────────────────
	fmt.Println(blue("Available self-tests:"))
	tests, err := client.GetAvailableSelfTests(ctx, devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: %v", err)))
	} else if len(tests.Available) == 0 {
		fmt.Println("  None reported by device")
	} else {
		for _, name := range tests.Available {
			if dur := tests.Durations[name]; dur > 0 {
				fmt.Printf("  - %-12s (~%d min)\n", name, dur)
			} else {
				fmt.Printf("  - %s\n", name)
			}
		}
	}
	fmt.Println()

	fmt.Println(green("✓ LibBackend example completed successfully"))
}
