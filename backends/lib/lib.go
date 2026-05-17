//go:build linux || darwin || freebsd

package lib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"unsafe"

	"github.com/dianlight/tlog"
	"github.com/ebitengine/purego"

	smtypes "github.com/dianlight/smartmontools-go/internal/types"
)

// abiVersion is the libsmartctl C ABI version this backend requires.
const abiVersion = 1

// defaultLibNames contains dynamic-linker-resolved names tried before absolute paths.
// The linker resolves versioned sonames, LD_LIBRARY_PATH, and rpath automatically.
var defaultLibNames = []string{
	"libsmartctl.so.0",
	"libsmartctl.so",
	"libsmartctl.dylib",
}

// defaultLibPaths contains platform-specific absolute paths tried as fallback.
var defaultLibPaths = []string{
	"/usr/local/lib/libsmartctl.so",
	"/usr/lib/libsmartctl.so",
	"/usr/lib/x86_64-linux-gnu/libsmartctl.so",
	"/usr/lib/aarch64-linux-gnu/libsmartctl.so",
	"/usr/local/lib/libsmartctl.so.0",
	"/opt/homebrew/lib/libsmartctl.dylib",
	"/usr/local/lib/libsmartctl.dylib",
	"/usr/local/lib/libsmartctl.so",
}

// libFuncs holds registered C function pointers loaded from libsmartctl.
type libFuncs struct {
	abiVersion    func() int32
	init          func(ctx *uintptr) int32
	destroy       func(ctx uintptr)
	setOption     func(ctx uintptr, key string, value string) int32
	scanDevices   func(ctx uintptr, outJSON *unsafe.Pointer) int32
	getSmartData  func(ctx uintptr, device string, outJSON *unsafe.Pointer) int32
	checkHealth   func(ctx uintptr, device string, outHealthy *int32) int32
	runSelfTest   func(ctx uintptr, device string, testType string) int32
	enableSMART   func(ctx uintptr, device string) int32
	disableSMART  func(ctx uintptr, device string) int32
	abortSelfTest func(ctx uintptr, device string) int32
	lastError     func(ctx uintptr) unsafe.Pointer // returns const char* owned by ctx
	freeString    func(s unsafe.Pointer)
}

// Option configures a LibBackend.
type Option func(*LibBackend)

// LibBackend implements Backend by loading libsmartctl via purego at runtime.
// All SMART operations are delegated to the shared library's C API without CGO.
type LibBackend struct {
	libHandle  uintptr
	ctx        uintptr
	libPath    string
	funcs      *libFuncs
	logHandler LogAdapter
	closeOnce  sync.Once
}

// Ensure LibBackend satisfies the Backend interface at compile time.
var _ Backend = (*LibBackend)(nil)

// WithLibraryPath sets a custom path to libsmartctl.so or libsmartctl.dylib.
// When not set, New searches defaultLibNames and defaultLibPaths in order.
func WithLibraryPath(path string) Option {
	return func(b *LibBackend) {
		b.libPath = path
	}
}

// WithSlogHandler sets a slog.Logger for the backend.
func WithSlogHandler(logger *slog.Logger) Option {
	return withLogHandler(logger)
}

// WithTLogHandler sets a tlog.Logger for the backend.
func WithTLogHandler(logger *tlog.Logger) Option {
	return withLogHandler(logger)
}

// WithLogHandler sets a custom logger adapter for the backend.
func WithLogHandler(logger LogAdapter) Option {
	return withLogHandler(logger)
}

func withLogHandler(logger LogAdapter) Option {
	return func(b *LibBackend) {
		b.logHandler = logger
	}
}

// New creates a LibBackend by loading libsmartctl from the resolved library path.
// It verifies the C ABI version and initialises a smartctl context.
func New(opts ...Option) (*LibBackend, error) {
	b := &LibBackend{
		logHandler: tlog.NewLoggerWithLevel(tlog.LevelDebug),
	}
	for _, opt := range opts {
		opt(b)
	}

	if b.libPath == "" {
		path, err := resolveLibPath()
		if err != nil {
			return nil, err
		}
		b.libPath = path
	}

	handle, err := purego.Dlopen(b.libPath, purego.RTLD_LAZY|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", b.libPath, err)
	}

	funcs, err := registerFuncs(handle)
	if err != nil {
		// Ignore close error; the primary error is more informative.
		_ = purego.Dlclose(handle)
		return nil, fmt.Errorf("failed to register library functions from %s: %w", b.libPath, err)
	}
	b.funcs = funcs

	got := funcs.abiVersion()
	if got != abiVersion {
		_ = purego.Dlclose(handle)
		return nil, fmt.Errorf("incompatible libsmartctl ABI: library reports version %d, backend requires %d", got, abiVersion)
	}

	var ctx uintptr
	if rc := funcs.init(&ctx); rc != 0 {
		_ = purego.Dlclose(handle)
		return nil, fmt.Errorf("smartctl_init failed (code %d)", rc)
	}
	b.libHandle = handle
	b.ctx = ctx
	return b, nil
}

// Name returns the backend identifier.
func (b *LibBackend) Name() string {
	return "lib"
}

// Close destroys the smartctl context and unloads the shared library.
// It is safe to call Close more than once; subsequent calls are no-ops.
func (b *LibBackend) Close() error {
	var closeErr error
	b.closeOnce.Do(func() {
		if b.ctx != 0 && b.funcs != nil {
			b.funcs.destroy(b.ctx)
			b.ctx = 0
		}
		if b.libHandle != 0 {
			if err := purego.Dlclose(b.libHandle); err != nil {
				closeErr = err
			}
			b.libHandle = 0
		}
	})
	return closeErr
}

// ScanDevices returns the list of storage devices visible to libsmartctl.
func (b *LibBackend) ScanDevices(ctx context.Context) ([]Device, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	jsonStr, err := b.callWithStringOut(func(out *unsafe.Pointer) int32 {
		return b.funcs.scanDevices(b.ctx, out)
	})
	if err != nil {
		return nil, fmt.Errorf("scan devices: %w", err)
	}

	var result struct {
		Devices []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"devices"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse scan output: %w", err)
	}

	devices := make([]Device, len(result.Devices))
	for i, d := range result.Devices {
		devices[i] = Device{Name: d.Name, Type: d.Type}
	}
	return devices, nil
}

// GetSMARTInfo returns comprehensive SMART information for the given device.
func (b *LibBackend) GetSMARTInfo(ctx context.Context, devicePath string) (*SMARTInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	jsonStr, err := b.getSmartJSON(devicePath)
	if err != nil {
		return nil, err
	}

	var info SMARTInfo
	if err := json.Unmarshal([]byte(jsonStr), &info); err != nil {
		return nil, fmt.Errorf("failed to parse SMART info for %s: %w", devicePath, err)
	}
	return &info, nil
}

// CheckHealth returns true when the device passes its SMART overall-health assessment.
func (b *LibBackend) CheckHealth(ctx context.Context, devicePath string) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}

	var healthy int32
	rc := b.funcs.checkHealth(b.ctx, devicePath, &healthy)
	if rc != 0 {
		return false, fmt.Errorf("check health for %s: %w", devicePath, b.lastError())
	}
	return healthy != 0, nil
}

// GetDeviceInfo returns raw key/value device information for the given device.
func (b *LibBackend) GetDeviceInfo(ctx context.Context, devicePath string) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	jsonStr, err := b.getSmartJSON(devicePath)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse device info for %s: %w", devicePath, err)
	}
	return result, nil
}

// RunSelfTest starts a SMART self-test of the given type on the device.
func (b *LibBackend) RunSelfTest(ctx context.Context, devicePath string, testType string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	rc := b.funcs.runSelfTest(b.ctx, devicePath, testType)
	if rc != 0 {
		return fmt.Errorf("run self-test (%s) on %s: %w", testType, devicePath, b.lastError())
	}
	return nil
}

// GetAvailableSelfTests returns the self-test types available on the device and
// their estimated durations. It parses the full SMART data for this information.
func (b *LibBackend) GetAvailableSelfTests(ctx context.Context, devicePath string) (*SelfTestInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	jsonStr, err := b.getSmartJSON(devicePath)
	if err != nil {
		return nil, err
	}

	var caps struct {
		AtaSmartData               *smtypes.AtaSmartData               `json:"ata_smart_data,omitempty"`
		NvmeControllerCapabilities *smtypes.NvmeControllerCapabilities `json:"nvme_controller_capabilities,omitempty"`
		NvmeOptionalAdminCommands  *smtypes.NvmeOptionalAdminCommands  `json:"nvme_optional_admin_commands,omitempty"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &caps); err != nil {
		return nil, fmt.Errorf("failed to parse capabilities for %s: %w", devicePath, err)
	}

	info := &SelfTestInfo{
		Available: []string{},
		Durations: make(map[string]int),
	}
	smtypes.PopulateSelfTestInfo(info, caps.AtaSmartData, caps.NvmeControllerCapabilities, caps.NvmeOptionalAdminCommands)
	return info, nil
}

// EnableSMART enables SMART on the given device.
func (b *LibBackend) EnableSMART(ctx context.Context, devicePath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	rc := b.funcs.enableSMART(b.ctx, devicePath)
	if rc != 0 {
		return fmt.Errorf("enable SMART on %s: %w", devicePath, b.lastError())
	}
	return nil
}

// DisableSMART disables SMART on the given device.
func (b *LibBackend) DisableSMART(ctx context.Context, devicePath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	rc := b.funcs.disableSMART(b.ctx, devicePath)
	if rc != 0 {
		return fmt.Errorf("disable SMART on %s: %w", devicePath, b.lastError())
	}
	return nil
}

// AbortSelfTest aborts a running SMART self-test on the given device.
func (b *LibBackend) AbortSelfTest(ctx context.Context, devicePath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	rc := b.funcs.abortSelfTest(b.ctx, devicePath)
	if rc != 0 {
		return fmt.Errorf("abort self-test on %s: %w", devicePath, b.lastError())
	}
	return nil
}

// resolveLibPath searches defaultLibNames (via the dynamic linker) and then
// defaultLibPaths (absolute locations) for a loadable libsmartctl library.
func resolveLibPath() (string, error) {
	// Try dynamic-linker-resolved names first (respects LD_LIBRARY_PATH, rpath, etc.).
	for _, name := range defaultLibNames {
		if h, err := purego.Dlopen(name, purego.RTLD_LAZY|purego.RTLD_LOCAL); err == nil {
			_ = purego.Dlclose(h)
			return name, nil
		}
	}

	// Fall back to well-known absolute paths.
	for _, path := range defaultLibPaths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		return path, nil
	}

	return "", errors.New(
		"libsmartctl not found. Build it with the D1 patch pipeline:\n" +
			"  https://github.com/dianlight/smartmontools-go/tree/main/patches\n" +
			"Then install the resulting libsmartctl.so to a location on LD_LIBRARY_PATH.",
	)
}

// registerFuncs uses purego to bind each C symbol from the loaded library.
// It returns an error if any required symbol cannot be resolved.
func registerFuncs(lib uintptr) (*libFuncs, error) {
	f := &libFuncs{}
	type binding struct {
		ptr  any
		name string
	}
	for _, b := range []binding{
		{&f.abiVersion, "smartctl_abi_version"},
		{&f.init, "smartctl_init"},
		{&f.destroy, "smartctl_destroy"},
		{&f.setOption, "smartctl_set_option"},
		{&f.scanDevices, "smartctl_scan_devices"},
		{&f.getSmartData, "smartctl_get_smart_data"},
		{&f.checkHealth, "smartctl_check_health"},
		{&f.runSelfTest, "smartctl_run_selftest"},
		{&f.enableSMART, "smartctl_enable_smart"},
		{&f.disableSMART, "smartctl_disable_smart"},
		{&f.abortSelfTest, "smartctl_abort_selftest"},
		{&f.lastError, "smartctl_last_error"},
		{&f.freeString, "smartctl_free_string"},
	} {
		if err := registerFunc(lib, b.name, b.ptr); err != nil {
			return nil, err
		}
	}
	return f, nil
}

// registerFunc registers a single C symbol, converting any panic into an error.
func registerFunc(lib uintptr, name string, fptr any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("symbol %q: %v", name, r)
		}
	}()
	purego.RegisterLibFunc(fptr, lib, name)
	return nil
}

// getSmartJSON calls smartctl_get_smart_data and returns the raw JSON string.
func (b *LibBackend) getSmartJSON(devicePath string) (string, error) {
	return b.callWithStringOut(func(out *unsafe.Pointer) int32 {
		return b.funcs.getSmartData(b.ctx, devicePath, out)
	})
}

// callWithStringOut calls a C function that allocates a JSON string via an
// output unsafe.Pointer parameter, reads the string into Go memory, and frees it.
func (b *LibBackend) callWithStringOut(fn func(out *unsafe.Pointer) int32) (string, error) {
	var outPtr unsafe.Pointer
	rc := fn(&outPtr)
	if rc != 0 {
		return "", b.lastError()
	}
	if outPtr == nil {
		return "", errors.New("library returned nil output")
	}
	// Read the C string into Go memory before freeing the C allocation.
	result := cGoString(outPtr)
	b.funcs.freeString(outPtr)
	return result, nil
}

// lastError reads the last error message from the C context as a Go error.
func (b *LibBackend) lastError() error {
	if b.funcs == nil {
		return errors.New("backend not initialised")
	}
	ptr := b.funcs.lastError(b.ctx)
	if ptr == nil {
		return errors.New("unknown library error")
	}
	return errors.New(cGoString(ptr))
}

// cGoString converts a null-terminated C string pointer into a Go string.
// The original C memory is not modified or freed.
func cGoString(p unsafe.Pointer) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(p), n))
}
