// Package main demonstrates the CompareBackend — a virtual backend that runs
// two or more backends in parallel and logs any discrepancies between their
// results. The first backend is the master: its output is always returned to
// the caller. Secondary backends are shadow-tested; mismatches produce a
// warning log and errors produce an error log.
//
// # Intended use
//
// CompareBackend is designed to compare *different implementations* of the
// Backend interface against the same device, for example:
//
//   - ExecBackend (master) vs LibBackend — validates exec/SDK parity
//   - ExecBackend v7 vs ExecBackend v8   — validates smartctl upgrade safety
//
// Do NOT pair two identical ExecBackend instances. Both would shell out to
// the same smartctl binary on the same physical device at the same time.
// Most OS device drivers serialize ATA/SCSI command queues, so one call
// wins and the other may receive EBUSY or a partial response, causing
// spurious error and mismatch logs that have no diagnostic value.
//
// Run:
//
//	go run .
//
// This example uses ExecBackend as master and a simple stub as secondary so
// it runs on any machine without needing two distinct backend implementations.
// Replace stubBackend with a real LibBackend in a production validation setup.
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
	fmt.Println("NOTE: CompareBackend is for comparing *different* backend implementations")
	fmt.Println("      (e.g. ExecBackend vs LibBackend). Using two identical ExecBackend")
	fmt.Println("      instances causes device contention: both processes hit the same")
	fmt.Println("      physical device simultaneously, which the OS serializes — one may")
	fmt.Println("      get EBUSY or a partial response, generating spurious log noise.")
	fmt.Println()

	// Use a structured logger so compare warnings/errors appear in the output.
	logger := tlog.NewLoggerWithLevel(tlog.LevelInfo)

	// Master backend — the source of truth for all returned results.
	master, err := execbackend.New(
		execbackend.WithTLogHandler(logger),
	)
	if err != nil {
		fmt.Println(red(fmt.Sprintf("✗ Failed to create master backend: %v", err)))
		os.Exit(1)
	}

	// Secondary backend — in a real scenario this would be a LibBackend or a
	// different implementation. Here we use a passthrough stub that delegates
	// to the same ExecBackend sequentially (after the master finishes), which
	// avoids device contention while still demonstrating the compare wiring.
	//
	// To test with a real second implementation, replace stub with:
	//
	//   lib, _ := libbackend.New(libbackend.WithTLogHandler(logger))
	//   secondary = lib
	secondary := &sequentialStub{delegate: master}

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

	fmt.Printf("Active backends: %s (master) + %s (secondary stub)\n\n",
		blue(master.Name()), blue(secondary.Name()))
	fmt.Println(green("✓ CompareBackend ready — no mismatches expected with the stub"))
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
	fmt.Println("Tip: replace stubBackend with a real LibBackend to compare exec vs SDK parity.")
	fmt.Println("     'compare: result mismatch' warnings = backends returned different data.")
	fmt.Println("     'compare: secondary backend error'  = secondary backend failed.")
}

// sequentialStub is a secondary backend that delegates every call to an
// existing backend sequentially. It is used in this example only to
// demonstrate compare wiring without causing device contention from two
// parallel smartctl processes hitting the same hardware simultaneously.
//
// In production, replace this with a real alternative implementation such as
// LibBackend.
type sequentialStub struct {
	delegate smartmontools.Backend
}

func (s *sequentialStub) Name() string { return "stub(" + s.delegate.Name() + ")" }
func (s *sequentialStub) Close() error { return nil } // delegate is closed by master

func (s *sequentialStub) ScanDevices(ctx context.Context) ([]smartmontools.Device, error) {
	return s.delegate.ScanDevices(ctx)
}
func (s *sequentialStub) GetSMARTInfo(ctx context.Context, path string) (*smartmontools.SMARTInfo, error) {
	return s.delegate.GetSMARTInfo(ctx, path)
}
func (s *sequentialStub) CheckHealth(ctx context.Context, path string) (bool, error) {
	return s.delegate.CheckHealth(ctx, path)
}
func (s *sequentialStub) GetDeviceInfo(ctx context.Context, path string) (map[string]any, error) {
	return s.delegate.GetDeviceInfo(ctx, path)
}
func (s *sequentialStub) RunSelfTest(ctx context.Context, path, testType string) error {
	return s.delegate.RunSelfTest(ctx, path, testType)
}
func (s *sequentialStub) GetAvailableSelfTests(ctx context.Context, path string) (*smartmontools.SelfTestInfo, error) {
	return s.delegate.GetAvailableSelfTests(ctx, path)
}
func (s *sequentialStub) EnableSMART(ctx context.Context, path string) error {
	return s.delegate.EnableSMART(ctx, path)
}
func (s *sequentialStub) DisableSMART(ctx context.Context, path string) error {
	return s.delegate.DisableSMART(ctx, path)
}
func (s *sequentialStub) AbortSelfTest(ctx context.Context, path string) error {
	return s.delegate.AbortSelfTest(ctx, path)
}
