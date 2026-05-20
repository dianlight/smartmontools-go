//go:build linux || darwin

package lib

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	smtypes "github.com/dianlight/smartmontools-go/internal/types"
)

var _ Backend = (*LibBackend)(nil)

// TestLibBackend_Name verifies that Name returns "lib".
func TestLibBackend_Name(t *testing.T) {
	b := &LibBackend{}
	assert.Equal(t, "lib", b.Name())
}

// TestLibBackend_Close_Idempotent verifies that Close can be called multiple times safely.
func TestLibBackend_Close_Idempotent(t *testing.T) {
	b := &LibBackend{}
	assert.NoError(t, b.Close())
	assert.NoError(t, b.Close())
}

// TestNew_InvalidPath verifies that New returns an error for a non-existent library path.
func TestNew_InvalidPath(t *testing.T) {
	_, err := New(WithLibraryPath("/nonexistent/libsmartmon_go.so"))
	require.Error(t, err)
}

// TestNew_MissingLibrary verifies that New returns an error when no library is found.
func TestNew_MissingLibrary(t *testing.T) {
	if _, err := New(); err == nil {
		t.Skip("smartmon wrapper library is installed; skipping missing-library test")
	}
	_, err := New()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestWithLogHandler verifies that the log handler option is applied without panic.
func TestWithLogHandler(t *testing.T) {
	b := &LibBackend{}
	WithLogHandler(nil)(b)
	assert.Nil(t, b.logHandler)
}

// newTestBackend constructs a LibBackend with injectable fake C functions, enabling
// unit testing of the Go wrapper logic without a real shared library.
func newTestBackend(t *testing.T, fns *libFuncs) *LibBackend {
	t.Helper()
	return &LibBackend{funcs: fns}
}

// makeCStr allocates a null-terminated byte slice in Go heap memory and returns
// its address as unsafe.Pointer, simulating a C string for fake C functions.
func makeCStr(t *testing.T, s string) unsafe.Pointer {
	t.Helper()
	b := make([]byte, len(s)+1)
	copy(b, s)
	t.Cleanup(func() { _ = b })
	if len(b) == 0 {
		return nil
	}
	return unsafe.Pointer(&b[0])
}

// fakeLastError returns a lastError function that always returns the given message.
func fakeLastError(t *testing.T, msg string) func() unsafe.Pointer {
	ptr := makeCStr(t, msg)
	return func() unsafe.Pointer { return ptr }
}

// TestScanDevices_Success verifies that ScanDevices parses a JSON device list.
func TestScanDevices_Success(t *testing.T) {
	scanJSON := `{"devices":[{"name":"/dev/sda","type":"ata"},{"name":"/dev/sdb","type":"nvme"}]}`
	b := newTestBackend(t, &libFuncs{
		scanDevices: func(out *unsafe.Pointer) int32 {
			*out = makeCStr(t, scanJSON)
			return 0
		},
		freeString: func(unsafe.Pointer) {},
		lastError:  fakeLastError(t, ""),
	})

	devices, err := b.ScanDevices(context.Background())
	require.NoError(t, err)
	require.Len(t, devices, 2)
	assert.Equal(t, Device{Name: "/dev/sda", Type: "ata"}, devices[0])
	assert.Equal(t, Device{Name: "/dev/sdb", Type: "nvme"}, devices[1])
}

// TestScanDevices_Error verifies that a non-zero return code yields an error.
func TestScanDevices_Error(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		scanDevices: func(_ *unsafe.Pointer) int32 { return 1 },
		lastError:   fakeLastError(t, "scan failed"),
	})

	_, err := b.ScanDevices(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan failed")
}

// TestScanDevices_ContextCancelled verifies early return when context is done.
func TestScanDevices_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := newTestBackend(t, &libFuncs{})
	_, err := b.ScanDevices(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

// TestGetSMARTInfo_Success verifies JSON parsing of full SMART data.
func TestGetSMARTInfo_Success(t *testing.T) {
	info := map[string]any{
		"device":        map[string]any{"name": "/dev/sda", "type": "ata"},
		"model_name":    "Test Drive",
		"serial_number": "SN123456",
	}
	raw, err := json.Marshal(info)
	require.NoError(t, err)

	b := newTestBackend(t, &libFuncs{
		getSmartData: func(_, _ string, out *unsafe.Pointer) int32 {
			*out = makeCStr(t, string(raw))
			return 0
		},
		freeString: func(unsafe.Pointer) {},
		lastError:  fakeLastError(t, ""),
	})

	got, err := b.GetSMARTInfo(context.Background(), "/dev/sda")
	require.NoError(t, err)
	assert.Equal(t, "Test Drive", got.ModelName)
	assert.Equal(t, "SN123456", got.SerialNumber)
}

// TestGetSMARTInfo_ContextCancelled verifies early return when context is done.
func TestGetSMARTInfo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := newTestBackend(t, &libFuncs{})
	_, err := b.GetSMARTInfo(ctx, "/dev/sda")
	require.ErrorIs(t, err, context.Canceled)
}

// TestGetSMARTInfo_Error verifies error propagation from C API.
func TestGetSMARTInfo_Error(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		getSmartData: func(_, _ string, _ *unsafe.Pointer) int32 { return 2 },
		lastError:    fakeLastError(t, "device not found"),
	})

	_, err := b.GetSMARTInfo(context.Background(), "/dev/sda")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "device not found")
}

// TestCheckHealth_Healthy verifies that healthy=1 maps to true.
func TestCheckHealth_Healthy(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		checkHealth: func(_, _ string, out *int32) int32 {
			*out = 1
			return 0
		},
		lastError: fakeLastError(t, ""),
	})

	ok, err := b.CheckHealth(context.Background(), "/dev/sda")
	require.NoError(t, err)
	assert.True(t, ok)
}

// TestCheckHealth_Unhealthy verifies that healthy=0 maps to false.
func TestCheckHealth_Unhealthy(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		checkHealth: func(_, _ string, out *int32) int32 {
			*out = 0
			return 0
		},
		lastError: fakeLastError(t, ""),
	})

	ok, err := b.CheckHealth(context.Background(), "/dev/sda")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestCheckHealth_Error verifies error propagation.
func TestCheckHealth_Error(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		checkHealth: func(_, _ string, _ *int32) int32 { return 1 },
		lastError:   fakeLastError(t, "ioctl error"),
	})

	_, err := b.CheckHealth(context.Background(), "/dev/sda")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ioctl error")
}

// TestGetDeviceInfo_Success verifies JSON parsing of device info.
func TestGetDeviceInfo_Success(t *testing.T) {
	payload := `{"model_name":"Test","firmware_version":"1.0"}`
	b := newTestBackend(t, &libFuncs{
		getSmartData: func(_, _ string, out *unsafe.Pointer) int32 {
			*out = makeCStr(t, payload)
			return 0
		},
		freeString: func(unsafe.Pointer) {},
		lastError:  fakeLastError(t, ""),
	})

	result, err := b.GetDeviceInfo(context.Background(), "/dev/sda")
	require.NoError(t, err)
	assert.Equal(t, "Test", result["model_name"])
	assert.Equal(t, "1.0", result["firmware_version"])
}

// TestRunSelfTest_Success verifies success path and argument forwarding.
func TestRunSelfTest_Success(t *testing.T) {
	var gotDevice, gotType string
	b := newTestBackend(t, &libFuncs{
		runSelftest: func(device, _, testType string) int32 {
			gotDevice = device
			gotType = testType
			return 0
		},
		lastError: fakeLastError(t, ""),
	})

	err := b.RunSelfTest(context.Background(), "/dev/sda", "short")
	require.NoError(t, err)
	assert.Equal(t, "/dev/sda", gotDevice)
	assert.Equal(t, "short", gotType)
}

// TestRunSelfTest_Error verifies error propagation.
func TestRunSelfTest_Error(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		runSelftest: func(_, _, _ string) int32 { return 1 },
		lastError:   fakeLastError(t, "self-test not supported"),
	})

	err := b.RunSelfTest(context.Background(), "/dev/sda", "short")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self-test not supported")
}

// TestEnableSMART_Success verifies success path.
func TestEnableSMART_Success(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		enableSmart: func(_, _ string) int32 { return 0 },
		lastError:   fakeLastError(t, ""),
	})
	assert.NoError(t, b.EnableSMART(context.Background(), "/dev/sda"))
}

// TestDisableSMART_Error verifies error propagation.
func TestDisableSMART_Error(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		disableSmart: func(_, _ string) int32 { return 1 },
		lastError:    fakeLastError(t, "NVMe does not support disable"),
	})
	err := b.DisableSMART(context.Background(), "/dev/nvme0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NVMe does not support disable")
}

// TestAbortSelfTest_Success verifies success path.
func TestAbortSelfTest_Success(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		abortSelftest: func(_, _ string) int32 { return 0 },
		lastError:     fakeLastError(t, ""),
	})
	assert.NoError(t, b.AbortSelfTest(context.Background(), "/dev/sda"))
}

// TestGetAvailableSelfTests_ATA verifies ATA self-test capability parsing via
// PopulateSelfTestInfo (pure Go logic, no library call required for this layer).
func TestGetAvailableSelfTests_ATA(t *testing.T) {
	payload := `{
		"ata_smart_data": {
			"capabilities": {"self_tests_supported": true, "conveyance_self_test_supported": true},
			"self_test": {
				"polling_minutes": {"short": 2, "extended": 120, "conveyance": 5}
			}
		}
	}`
	b := newTestBackend(t, &libFuncs{
		getSmartData: func(_, _ string, out *unsafe.Pointer) int32 {
			*out = makeCStr(t, payload)
			return 0
		},
		freeString: func(unsafe.Pointer) {},
		lastError:  fakeLastError(t, ""),
	})

	info, err := b.GetAvailableSelfTests(context.Background(), "/dev/sda")
	require.NoError(t, err)
	assert.Contains(t, info.Available, "short")
	assert.Contains(t, info.Available, "long")
	assert.Equal(t, 2, info.Durations["short"])
	assert.Equal(t, 120, info.Durations["long"])
}

// TestPopulateSelfTestInfo_ATA tests the pure Go self-test info parsing directly.
func TestPopulateSelfTestInfo_ATA(t *testing.T) {
	pm := smtypes.PollingMinutes{Short: 1, Extended: 60, Conveyance: 3}
	st := smtypes.SelfTest{PollingMinutes: &pm}
	caps := smtypes.Capabilities{SelfTestsSupported: true, ConveyanceSelfTestSupported: true}
	ata := &smtypes.AtaSmartData{
		Capabilities: &caps,
		SelfTest:     &st,
	}
	info := &smtypes.SelfTestInfo{
		Available: []string{},
		Durations: make(map[string]int),
	}
	smtypes.PopulateSelfTestInfo(info, ata, nil, nil)

	assert.Contains(t, info.Available, "short")
	assert.Contains(t, info.Available, "long")
	assert.Contains(t, info.Available, "conveyance")
	assert.Equal(t, 1, info.Durations["short"])
	assert.Equal(t, 60, info.Durations["long"])
	assert.Equal(t, 3, info.Durations["conveyance"])
}

// TestGoString verifies the null-terminated C string decoder.
func TestGoString(t *testing.T) {
	cases := []struct {
		name string
		s    string
	}{
		{"empty", ""},
		{"simple", "hello"},
		{"version", "smartmon-7.5"},
		{"with spaces", "/dev/sda type=ata"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, len(tc.s)+1)
			copy(buf, tc.s)
			var ptr unsafe.Pointer
			if len(tc.s) > 0 {
				ptr = unsafe.Pointer(&buf[0])
			}
			assert.Equal(t, tc.s, goString(ptr))
		})
	}
}

// TestGoString_Nil verifies that a nil pointer returns an empty string.
func TestGoString_Nil(t *testing.T) {
	assert.Equal(t, "", goString(nil))
}

// TestLastError_NoFuncs verifies that lastError on an uninitialised backend returns a sensible error.
func TestLastError_NoFuncs(t *testing.T) {
	b := &LibBackend{}
	err := b.lastError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialised")
}

// TestCallWithStringOut_NilOutput verifies that a nil output pointer is reported as an error.
func TestCallWithStringOut_NilOutput(t *testing.T) {
	b := newTestBackend(t, &libFuncs{
		freeString: func(unsafe.Pointer) {},
		lastError:  fakeLastError(t, ""),
	})

	_, err := b.callWithStringOut(func(out *unsafe.Pointer) int32 {
		// Deliberately do not write to *out to simulate a library bug.
		return 0
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil output")
}

// TestIntegration_ScanDevices is an integration test that runs only when
// SMARTMON_LIB_PATH is set to a real smartmon wrapper library path.
func TestIntegration_ScanDevices(t *testing.T) {
	libPath := integrationLibPath(t)
	b, err := New(WithLibraryPath(libPath))
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	devices, err := b.ScanDevices(context.Background())
	require.NoError(t, err)
	t.Logf("found %d device(s)", len(devices))
	for _, d := range devices {
		assert.NotEmpty(t, d.Name)
	}
}

// TestIntegration_GetSMARTInfo is an integration test that exercises the full
// SMART data path against a real device. Set SMARTMON_SDK_DEVICE to run.
func TestIntegration_GetSMARTInfo(t *testing.T) {
	libPath := integrationLibPath(t)
	devicePath, ok := os.LookupEnv("SMARTMON_SDK_DEVICE")
	if !ok {
		t.Skip("set SMARTMON_SDK_DEVICE to run SMART data integration tests")
	}

	b, err := New(WithLibraryPath(libPath))
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	info, err := b.GetSMARTInfo(context.Background(), devicePath)
	require.NoError(t, err)
	assert.NotEmpty(t, info.ModelName)
	assert.NotEmpty(t, info.SerialNumber)
	t.Logf("model=%s serial=%s", info.ModelName, info.SerialNumber)
}

// integrationLibPath returns the wrapper library path from SMARTMON_LIB_PATH,
// or skips the test when the variable is not set.
func integrationLibPath(t *testing.T) string {
	t.Helper()
	path, ok := os.LookupEnv("SMARTMON_LIB_PATH")
	if !ok {
		t.Skip("set SMARTMON_LIB_PATH to run integration tests")
	}
	return path
}

