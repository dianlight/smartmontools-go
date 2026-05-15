package smartmontools

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMinimalClient creates a Client wired to a no-op mock commander so helper
// tests can run without a real smartctl binary.
func newMinimalClient(t *testing.T) *Client {
	t.Helper()
	client, err := NewClient(
		WithSmartctlPath("/usr/sbin/smartctl"),
		WithCommander(&mockCommander{cmds: map[string]*mockCmd{}}),
	)
	require.NoError(t, err)
	return client.(*Client)
}

// newMinimalTestLogger returns a discarding slog.Logger suitable for unit tests
// that do not need to inspect log output.
func newMinimalTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

// ─── resolveCtx ──────────────────────────────────────────────────────────────

func TestResolveCtx_NilReturnsDefault(t *testing.T) {
	c := newMinimalClient(t)
	type ctxKey struct{}
	sentinel := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	c.defaultCtx = sentinel

	var nilCtx context.Context
	got := c.resolveCtx(nilCtx)
	assert.Equal(t, sentinel, got, "nil ctx should fall back to defaultCtx")
}

func TestResolveCtx_NonNilPassthrough(t *testing.T) {
	c := newMinimalClient(t)
	type ctxKey struct{}
	explicit := context.WithValue(context.Background(), ctxKey{}, "explicit")

	got := c.resolveCtx(explicit)
	assert.Equal(t, explicit, got, "non-nil ctx should be returned unchanged")
}

// ─── buildArgs ───────────────────────────────────────────────────────────────

func TestBuildArgs_ColdCache(t *testing.T) {
	c := newMinimalClient(t)
	got := c.buildArgs("/dev/sda", "-a", "-j")
	assert.Equal(t, []string{"-a", "-j", "--nocheck=standby", "/dev/sda"}, got)
}

func TestBuildArgs_CachedATA(t *testing.T) {
	c := newMinimalClient(t)
	c.setCachedDeviceType("/dev/sda", "ata")
	got := c.buildArgs("/dev/sda", "-a", "-j")
	assert.Equal(t, []string{"-a", "-j", "--nocheck=standby", "-d", "ata", "/dev/sda"}, got)
}

func TestBuildArgs_CachedSAT(t *testing.T) {
	c := newMinimalClient(t)
	c.setCachedDeviceType("/dev/sda", "sat")
	got := c.buildArgs("/dev/sda", "-a", "-j")
	assert.Equal(t, []string{"-a", "-j", "--nocheck=standby", "-d", "sat", "/dev/sda"}, got,
		"SAT (USB-to-ATA bridge) should be treated as ATA and get --nocheck=standby")
}

func TestBuildArgs_CachedNVMe(t *testing.T) {
	c := newMinimalClient(t)
	c.setCachedDeviceType("/dev/nvme0", "nvme")
	got := c.buildArgs("/dev/nvme0", "-a", "-j")
	assert.Equal(t, []string{"-a", "-j", "-d", "nvme", "/dev/nvme0"}, got,
		"NVMe devices should not get --nocheck=standby")
}

func TestBuildArgs_MultipleFlags(t *testing.T) {
	c := newMinimalClient(t)
	got := c.buildArgs("/dev/sda", "-c", "-j")
	assert.Equal(t, []string{"-c", "-j", "--nocheck=standby", "/dev/sda"}, got,
		"all leading flags must appear in output")
}

// ─── logSmartctlMessages ─────────────────────────────────────────────────────

func TestLogSmartctlMessages_NilSmartctl(t *testing.T) {
	c := newMinimalClient(t)
	assert.NotPanics(t, func() {
		c.logSmartctlMessages(context.Background(), &SMARTInfo{})
	})
}

func TestLogSmartctlMessages_SeverityRouting(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := &Client{
		logHandler:      logger,
		deviceTypeCache: make(map[string]string),
		defaultCtx:      context.Background(),
	}

	// Prefix with t.Name() to keep global TTL cache from absorbing these messages
	// from other test runs within the same process.
	prefix := t.Name()
	info := &SMARTInfo{
		Smartctl: &SmartctlInfo{
			Messages: []Message{
				{String: prefix + "_error", Severity: "error"},
				{String: prefix + "_warning", Severity: "warning"},
				{String: prefix + "_info", Severity: "information"},
				{String: prefix + "_default", Severity: ""},
			},
		},
	}

	c.logSmartctlMessages(context.Background(), info)

	logged := buf.String()
	assert.Contains(t, logged, "ERROR", "error-severity message should be logged at ERROR")
	assert.Contains(t, logged, prefix+"_error")
	assert.Contains(t, logged, "WARN", "warning-severity message should be logged at WARN")
	assert.Contains(t, logged, prefix+"_warning")
	assert.Contains(t, logged, prefix+"_info", "information-severity message should be logged")
	assert.Contains(t, logged, prefix+"_default", "empty-severity message should be logged as INFO")
}

func TestLogSmartctlMessages_Deduplication(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := &Client{
		logHandler:      logger,
		deviceTypeCache: make(map[string]string),
		defaultCtx:      context.Background(),
	}

	msg := t.Name() + "_dedup_msg"
	info := &SMARTInfo{
		Smartctl: &SmartctlInfo{
			Messages: []Message{{String: msg, Severity: "information"}},
		},
	}

	c.logSmartctlMessages(context.Background(), info)
	firstLen := buf.Len()
	require.Positive(t, firstLen, "first call should log the message")

	c.logSmartctlMessages(context.Background(), info)
	assert.Equal(t, firstLen, buf.Len(),
		"second call within the TTL window should not re-log the same message")
}

// ─── populateSelfTestInfo ────────────────────────────────────────────────────

func TestPopulateSelfTestInfo_ATAFull(t *testing.T) {
	info := &SelfTestInfo{Available: []string{}, Durations: make(map[string]int)}
	populateSelfTestInfo(info, &AtaSmartData{
		Capabilities: &Capabilities{
			SelfTestsSupported:          true,
			ConveyanceSelfTestSupported: true,
			ExecOfflineImmediate:        true,
		},
		SelfTest: &SelfTest{
			PollingMinutes: &PollingMinutes{Short: 2, Extended: 48, Conveyance: 5},
		},
	}, nil, nil)

	assert.Equal(t, []string{"short", "long", "conveyance", "offline"}, info.Available)
	assert.Equal(t, map[string]int{"short": 2, "long": 48, "conveyance": 5}, info.Durations)
}

func TestPopulateSelfTestInfo_ATANoSelfTestBlock(t *testing.T) {
	info := &SelfTestInfo{Available: []string{}, Durations: make(map[string]int)}
	populateSelfTestInfo(info, &AtaSmartData{
		Capabilities: &Capabilities{SelfTestsSupported: true},
		// SelfTest is nil — no polling-minute data
	}, nil, nil)

	assert.Equal(t, []string{"short", "long"}, info.Available)
	assert.Empty(t, info.Durations)
}

func TestPopulateSelfTestInfo_ATANilCapabilities(t *testing.T) {
	info := &SelfTestInfo{Available: []string{}, Durations: make(map[string]int)}
	populateSelfTestInfo(info, &AtaSmartData{Capabilities: nil}, nil, nil)

	assert.Empty(t, info.Available)
	assert.Empty(t, info.Durations)
}

func TestPopulateSelfTestInfo_NVMeViaCaps(t *testing.T) {
	info := &SelfTestInfo{Available: []string{}, Durations: make(map[string]int)}
	populateSelfTestInfo(info, nil, &NvmeControllerCapabilities{SelfTest: true}, nil)

	assert.Equal(t, []string{"short"}, info.Available)
	assert.Empty(t, info.Durations)
}

func TestPopulateSelfTestInfo_NVMeViaOptional(t *testing.T) {
	info := &SelfTestInfo{Available: []string{}, Durations: make(map[string]int)}
	populateSelfTestInfo(info, nil, nil, &NvmeOptionalAdminCommands{SelfTest: true})

	assert.Equal(t, []string{"short"}, info.Available)
	assert.Empty(t, info.Durations)
}

func TestPopulateSelfTestInfo_NVMeBothFieldsOnceShort(t *testing.T) {
	info := &SelfTestInfo{Available: []string{}, Durations: make(map[string]int)}
	populateSelfTestInfo(info,
		nil,
		&NvmeControllerCapabilities{SelfTest: true},
		&NvmeOptionalAdminCommands{SelfTest: true},
	)

	assert.Equal(t, []string{"short"}, info.Available,
		"both NVMe capability fields should produce exactly one 'short' entry")
}

func TestPopulateSelfTestInfo_AllNil(t *testing.T) {
	info := &SelfTestInfo{Available: []string{}, Durations: make(map[string]int)}
	populateSelfTestInfo(info, nil, nil, nil)

	assert.Empty(t, info.Available)
	assert.Empty(t, info.Durations)
}

// ─── defaultCommander field ──────────────────────────────────────────────────

// TestWithCommander_SetsDefaultCommanderFalse verifies that providing a custom
// commander via WithCommander sets the defaultCommander flag to false, which in
// turn skips the real-binary compatibility check in NewClient.
func TestWithCommander_SetsDefaultCommanderFalse(t *testing.T) {
	mock := &mockCommander{cmds: map[string]*mockCmd{}}
	client, err := NewClient(
		WithSmartctlPath("/usr/sbin/smartctl"),
		WithCommander(mock),
	)
	require.NoError(t, err)
	c := client.(*Client)

	assert.False(t, c.defaultCommander,
		"WithCommander should set defaultCommander=false")
	assert.Equal(t, mock, c.commander,
		"commander should be the provided mock")
}

// TestNewClient_DefaultCommanderTrue verifies that a client created without
// WithCommander has defaultCommander=true. Skipped when smartctl is absent.
func TestNewClient_DefaultCommanderTrue(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skipf("smartctl not available: %v", err)
	}
	c := client.(*Client)
	assert.True(t, c.defaultCommander,
		"client created without WithCommander should have defaultCommander=true")
}

// ─── retrySATFallback ────────────────────────────────────────────────────────

const satFallbackDevice = "/dev/sata1"

const satFallbackJSON = `{
	"json_format_version": [1, 0],
	"smartctl": {"version": [7, 5], "exit_status": 0},
	"device": {"name": "/dev/sata1", "type": "sat"},
	"model_name": "SAT Test Drive",
	"smart_status": {"passed": true}
}`

// TestGetSMARTInfo_SATFallback_Success verifies that when the initial smartctl
// call fails with execution-failure exit bits and no cached device type exists,
// a SAT-protocol retry is attempted and its successful result is returned.
func TestGetSMARTInfo_SATFallback_Success(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			// Initial call (cold cache — no -d flag, with --nocheck=standby) fails.
			"/usr/sbin/smartctl -a -j --nocheck=standby " + satFallbackDevice: {
				err: &exec.ExitError{},
			},
			// SAT retry (with --nocheck=standby and explicit -d sat) succeeds.
			"/usr/sbin/smartctl -a -j --nocheck=standby -d sat " + satFallbackDevice: {
				output: []byte(satFallbackJSON),
			},
		},
	}
	client, err := NewClient(
		WithSmartctlPath("/usr/sbin/smartctl"),
		WithCommander(commander),
	)
	require.NoError(t, err)

	info, err := client.GetSMARTInfo(context.Background(), satFallbackDevice)
	require.NoError(t, err)
	assert.Equal(t, satFallbackDevice, info.Device.Name)
	assert.Equal(t, "SAT Test Drive", info.ModelName)

	// Verify "sat" has been cached for subsequent calls.
	c := client.(*Client)
	cachedType, hasCached := c.getCachedDeviceType(satFallbackDevice)
	assert.True(t, hasCached, "device type should be cached after SAT fallback")
	assert.Equal(t, "sat", cachedType)
}

// TestGetSMARTInfo_SATFallback_SkippedWhenCached verifies that the SAT retry
// is NOT performed when a device type is already present in the cache.
func TestGetSMARTInfo_SATFallback_SkippedWhenCached(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sata1", "type": "sat"},
		"model_name": "Cached Drive",
		"smart_status": {"passed": true}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			// Cached "sat" type causes buildArgs to emit --nocheck=standby -d sat.
			// Only this key is present; if the retry path fires incorrectly it
			// will fail with "mock command not configured".
			"/usr/sbin/smartctl -a -j --nocheck=standby -d sat " + satFallbackDevice: {
				output: []byte(mockJSON),
			},
		},
	}
	client, err := NewClient(
		WithSmartctlPath("/usr/sbin/smartctl"),
		WithCommander(commander),
	)
	require.NoError(t, err)

	c := client.(*Client)
	c.setCachedDeviceType(satFallbackDevice, "sat")

	info, err := client.GetSMARTInfo(context.Background(), satFallbackDevice)
	require.NoError(t, err)
	assert.Equal(t, "Cached Drive", info.ModelName)
}

// TestRetrySATFallback_DirectCall_Success exercises retrySATFallback directly
// and verifies that a successful SAT response is returned with the type cached.
func TestRetrySATFallback_DirectCall_Success(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j --nocheck=standby -d sat " + satFallbackDevice: {
				output: []byte(satFallbackJSON),
			},
		},
	}
	c := newMinimalClient(t)
	c.commander = commander

	info, ok := c.retrySATFallback(context.Background(), satFallbackDevice)
	require.True(t, ok, "retrySATFallback should report success")
	assert.Equal(t, satFallbackDevice, info.Device.Name)
	assert.Equal(t, "SAT Test Drive", info.ModelName)

	cachedType, hasCached := c.getCachedDeviceType(satFallbackDevice)
	assert.True(t, hasCached, "device type should be cached after successful SAT fallback")
	assert.Equal(t, "sat", cachedType)
}

// TestRetrySATFallback_DirectCall_FallsThrough verifies that retrySATFallback
// returns (nil, false) when the SAT retry command itself fails.
func TestRetrySATFallback_DirectCall_FallsThrough(t *testing.T) {
	// No commands configured — the SAT command returns "mock command not configured".
	c := newMinimalClient(t)

	info, ok := c.retrySATFallback(context.Background(), satFallbackDevice)
	assert.False(t, ok, "retrySATFallback should return false when SAT also fails")
	assert.Nil(t, info)
}
