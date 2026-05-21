// Package main demonstrates the CompareBackend — a virtual backend that runs
// two or more backends in parallel and logs any discrepancies between their
// results. The first backend is the master: its output is always returned to
// the caller. Secondary backends are shadow-tested; mismatches produce a
// warning log and errors produce an error log.
//
// Run:
//
//	go run .
//
// The example creates two ExecBackend instances (both pointing at the same
// smartctl binary) so the results should always agree. Swap the secondary for
// a LibBackend or a differently-configured ExecBackend to exercise real
// comparison.
package main

import (
	"context"
	"fmt"
	"os"

	smartmontools "github.com/dianlight/smartmontools-go"
	comparebackend "github.com/dianlight/smartmontools-go/backends/compare"
	execbackend "github.com/dianlight/smartmontools-go/backends/exec"
	"github.com/dianlight/tlog"
	"github.com/fatih/color"
)

func main() {
	blue := color.New(color.FgBlue).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Println(blue("Smartmontools CompareBackend Example"))
	fmt.Println(blue("====================================="))
	fmt.Println()

	// Use a structured logger so compare warnings/errors appear in the output.
	logger := tlog.NewLoggerWithLevel(tlog.LevelDebug)

	// Master backend — the source of truth for all returned results.
	master, err := execbackend.New(
		execbackend.WithTLogHandler(logger),
	)
	if err != nil {
		fmt.Println(red(fmt.Sprintf("✗ Failed to create master backend: %v", err)))
		os.Exit(1)
	}

	// Secondary backend — runs in parallel; its results are compared against
	// the master. In production, this would typically be a LibBackend or a
	// differently-configured backend to validate parity.
	//
	// Here we reuse the same ExecBackend configuration so results should always
	// agree, producing no warnings.
	secondary, err := execbackend.New(
		execbackend.WithTLogHandler(logger),
	)
	if err != nil {
		fmt.Println(red(fmt.Sprintf("✗ Failed to create secondary backend: %v", err)))
		os.Exit(1)
	}

	// Wrap both backends in the compare backend.
	// Additional backends can be appended to the slice for broader coverage.
	compare, err := comparebackend.NewCompareBackend(
		[]smartmontools.Backend{master, secondary},
		comparebackend.WithTLogHandler(logger),
	)
	if err != nil {
		fmt.Println(red(fmt.Sprintf("✗ Failed to create compare backend: %v", err)))
		os.Exit(1)
	}
	defer func() {
		if err := compare.Close(); err != nil {
			tlog.Warn("Failed to close compare backend", "error", err)
		}
	}()

	fmt.Printf("Active backends: %s (master) + %s (secondary)\n\n",
		blue(master.Name()), blue(secondary.Name()))
	fmt.Println(green("✓ CompareBackend ready"))
	fmt.Println()

	// Wire the compare backend into the standard client API.
	client, err := smartmontools.NewClient(smartmontools.WithBackend(compare))
	if err != nil {
		tlog.Fatal("Failed to create client", "error", err)
	}

	ctx := context.Background()

	// ── Device discovery ──────────────────────────────────────────────────
	fmt.Println(blue("Scanning for devices (both backends run in parallel)..."))
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

	// ── SMART information ─────────────────────────────────────────────────
	fmt.Println(blue("Getting SMART information..."))
	smartInfo, err := client.GetSMARTInfo(ctx, devicePath)
	if err != nil {
		fmt.Println(yellow(fmt.Sprintf("Warning: %v", err)))
	} else {
		fmt.Printf("  Model:     %s\n", smartInfo.ModelName)
		fmt.Printf("  Serial:    %s\n", smartInfo.SerialNumber)
		fmt.Printf("  Firmware:  %s\n", smartInfo.Firmware)
		if smartInfo.DiskType != "" {
			fmt.Printf("  Disk Type: %s\n", smartInfo.DiskType)
		}
		if smartInfo.Temperature != nil {
			fmt.Printf("  Temp:      %d°C\n", smartInfo.Temperature.Current)
		}
		if smartInfo.PowerOnTime != nil {
			fmt.Printf("  Power-on:  %d hours\n", smartInfo.PowerOnTime.Hours)
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

	fmt.Println(green("✓ CompareBackend example completed successfully"))
	fmt.Println()
	fmt.Println("Tip: any 'compare: result mismatch' warnings above indicate")
	fmt.Println("     that the two backends returned different data for the same query.")
	fmt.Println("     'compare: secondary backend error' entries indicate a secondary failure.")
}
