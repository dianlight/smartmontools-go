# USB Bridge Device Type Database (drivedb_addendum)

## Overview

The `drivedb_addendum.txt` file is an embedded database of known USB bridge devices and their compatible device types. This database is used to automatically handle "Unknown USB bridge" errors by providing appropriate device type parameters (typically `-d sat`) without manual intervention.

## Purpose

When smartctl encounters an unknown USB bridge, it cannot automatically determine which protocol to use for communication. The `drivedb_addendum.txt` file provides a mapping of USB vendor:product IDs to device types, allowing the library to:

1. Automatically detect the correct device type for known USB bridges
2. Avoid unnecessary fallback attempts
3. Provide faster access to SMART data for common USB storage adapters
4. Reduce error messages and improve user experience

## File Format

The file uses a simple text format:

```
# Comments start with # and are ignored
# Empty lines are also ignored

# Format: usb:<vendor_id>:<product_id> <device_type>
usb:0x152d:0x578e sat
usb:0x0bda:0x9201 sat
```

### Fields

- **usb:<vendor_id>:<product_id>**: USB device identifier
  - `vendor_id`: 4-digit hexadecimal vendor ID (e.g., `0x152d`)
  - `product_id`: 4-digit hexadecimal product ID (e.g., `0x578e`)
  - IDs are case-insensitive (will be normalized to lowercase)

- **device_type**: smartctl device type parameter
  - Typically `sat` for SATA devices over USB
  - Other possible values: `usbjmicron`, `usbsunplus`, `usbcypress`, etc.

## How It Works

1. **Initialization**: When a client is created (`NewClient()` or `NewClientWithPath()`), the embedded `drivedb_addendum.txt` file is parsed and loaded into an in-memory cache.

2. **Detection**: When smartctl reports an "Unknown USB bridge" error, the library extracts the USB vendor:product ID from the error message.

3. **Lookup**: The extracted ID is checked against the prepopulated cache.

4. **Fallback**: 
   - If found in the addendum, the corresponding device type is used immediately
   - If not found, the library tries `-d sat` as a default fallback
   - Successful attempts are cached by device path for future calls

## Data Source

The entries in `drivedb_addendum.txt` are sourced from:

- Open issues on https://github.com/smartmontools/smartmontools/issues labeled "drivedb"
- Community-reported USB bridge devices that work with specific device types
- Official smartmontools drivedb updates

## Examples

### Example 1: Known USB Bridge (Intenso Memory Center)

```go
client, _ := smartmontools.NewClient()

// Device path with Intenso Memory Center (0x152d:0x578e)
// This USB bridge is in drivedb_addendum.txt
info, err := client.GetSMARTInfo("/dev/disk/by-id/usb-Intenso_xxx")

// The library automatically:
// 1. Detects "Unknown USB bridge [0x152d:0x578e]" error
// 2. Finds it in drivedb_addendum with device type "sat"
// 3. Retries with -d sat
// 4. Returns SMART info successfully
```

### Example 2: Unknown USB Bridge (Not in Database)

```go
client, _ := smartmontools.NewClient()

// Device with a USB bridge not in the addendum
info, err := client.GetSMARTInfo("/dev/disk/by-id/usb-NewBridge_xxx")

// The library automatically:
// 1. Detects "Unknown USB bridge [0xXXXX:0xYYYY]" error
// 2. Doesn't find it in drivedb_addendum
// 3. Falls back to trying -d sat (common for most USB-SATA bridges)
// 4. If successful, caches the device type for this device path
```

## Updating the Database

The `drivedb_addendum.txt` file is embedded in the binary at compile time. To add new USB bridges:

1. Edit `drivedb_addendum.txt` in the source repository
2. Add the USB bridge entry in the format: `usb:<vendor>:<product> <type>`
3. Rebuild the library

Example addition:
```
# My New USB Bridge - works with sat
usb:0x1234:0x5678 sat
```

## Thread Safety

The device type cache is protected by a `sync.RWMutex`, making it safe for concurrent access from multiple goroutines.

## Logging

The library provides debug and info logs when using the drivedb addendum:

- **Info**: When a USB bridge is found in the addendum
- **Info**: When successfully accessing a device with the determined device type
- **Debug**: Number of entries loaded from the addendum
- **Debug**: When caching device types

Enable debug logging to see detailed information about USB bridge detection and caching.

## Benefits

1. **Automatic**: No manual device type specification needed for known USB bridges
2. **Fast**: Immediate device type selection from prepopulated cache
3. **Comprehensive**: Includes common USB bridges from community reports
4. **Extensible**: New entries can be added to the embedded file
5. **Zero Configuration**: Works out of the box after library installation

## Related Functions

- `loadDrivedbAddendum()`: Parses and loads the embedded file
- `extractUSBBridgeID()`: Extracts USB vendor:product ID from error messages
- `getCachedDeviceType()`: Retrieves device type from cache
- `setCachedDeviceType()`: Stores device type in cache

## See Also

- [smartmontools GitHub Issues](https://github.com/smartmontools/smartmontools/issues?q=label%3Adrivedb)
- [smartmontools drivedb.h](https://github.com/smartmontools/smartmontools/blob/master/drivedb.h)
- [smartctl man page](https://www.smartmontools.org/browser/trunk/smartmontools/smartctl.8.in)
