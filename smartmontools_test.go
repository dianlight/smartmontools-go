package smartmontools

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// mockCmd implements Cmd interface for testing
type mockCmd struct {
	output []byte
	err    error
}

func (m *mockCmd) Output() ([]byte, error) {
	return m.output, m.err
}

func (m *mockCmd) Run() error {
	return m.err
}

// mockCommander implements Commander interface for testing
type mockCommander struct {
	cmds map[string]*mockCmd
}

func (m *mockCommander) Command(name string, arg ...string) Cmd {
	key := name
	for _, a := range arg {
		key += " " + a
	}
	if cmd, ok := m.cmds[key]; ok {
		return cmd
	}
	// Default mock command that returns error
	return &mockCmd{err: errors.New("mock command not configured")}
}

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skipf("smartctl not usable (missing or incompatible): %v", err)
	}

	c := client.(*Client)
	if c.smartctlPath == "" {
		t.Error("Expected smartctlPath to be set")
	}
}

func TestParseSmartctlVersion(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		major   int
		minor   int
		wantErr bool
	}{
		{
			name:  "linux typical",
			input: "smartctl 7.3 2022-02-28 r5338 [x86_64-linux] (local build)\n...",
			major: 7, minor: 3,
		},
		{
			name:  "mac typical",
			input: "smartctl 7.4 2023-12-30 r5678 (db:7.4/5678)\n...",
			major: 7, minor: 4,
		},
		{
			name:    "no match",
			input:   "some random output",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			major, minor, err := parseSmartctlVersion(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if major != tc.major || minor != tc.minor {
				t.Fatalf("expected %d.%d, got %d.%d", tc.major, tc.minor, major, minor)
			}
		})
	}
}

func TestNewClientWithPath(t *testing.T) {
	testPath := "/usr/sbin/smartctl"
	client := NewClientWithPath(testPath)

	c := client.(*Client)
	if c.smartctlPath != testPath {
		t.Errorf("Expected smartctlPath to be %s, got %s", testPath, c.smartctlPath)
	}
}

func TestScanDevices(t *testing.T) {
	mockJSON := `{
		"devices": [
			{"name": "/dev/sda", "type": "ata"},
			{"name": "/dev/sdb", "type": "ata"}
		]
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl --scan-open --json": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	devices, err := client.ScanDevices()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(devices))
	}

	if devices[0].Name != "/dev/sda" || devices[0].Type != "ata" {
		t.Errorf("Unexpected device 0: %+v", devices[0])
	}
}

func TestScanDevicesError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl --scan-open --json": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	_, err := client.ScanDevices()
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestGetSMARTInfo(t *testing.T) {
	mockJSON := `{
  "json_format_version": [
    1,
    0
  ],
  "smartctl": {
    "version": [
      7,
      5
    ],
    "pre_release": false,
    "svn_revision": "5714",
    "platform_info": "x86_64-linux-6.12.43-haos",
    "build_info": "(local build)",
    "argv": [
      "smartctl",
      "-a",
      "-j",
      "/dev/sda"
    ],
    "drive_database_version": {
      "string": "7.5/5706"
    },
    "exit_status": 0,
    "messages": [
      {"string": "Test informational message", "severity": "info"}
    ]
  },
  "local_time": {
    "time_t": 1762080587,
    "asctime": "Sun Nov  2 11:49:47 2025 CET"
  },
  "device": {
    "name": "/dev/sda",
    "info_name": "/dev/sda [SAT]",
    "type": "sat",
    "protocol": "ATA"
  },
  "model_family": "SandForce Driven SSDs",
  "model_name": "KINGSTON SV300S37A240G",
  "serial_number": "50026B77560145CF",
  "wwn": {
    "naa": 5,
    "oui": 9911,
    "id": 31507695055
  },
  "firmware_version": "603ABBF0",
  "user_capacity": {
    "blocks": 468862128,
    "bytes": 240057409536
  },
  "logical_block_size": 512,
  "physical_block_size": 512,
  "rotation_rate": 0,
  "trim": {
    "supported": true,
    "deterministic": false,
    "zeroed": false
  },
  "in_smartctl_database": true,
  "ata_version": {
    "string": "ATA8-ACS, ACS-2 T13/2015-D revision 3",
    "major_value": 508,
    "minor_value": 272
  },
  "sata_version": {
    "string": "SATA 3.0",
    "value": 63
  },
  "interface_speed": {
    "max": {
      "sata_value": 14,
      "string": "6.0 Gb/s",
      "units_per_second": 60,
      "bits_per_unit": 100000000
    },
    "current": {
      "sata_value": 3,
      "string": "6.0 Gb/s",
      "units_per_second": 60,
      "bits_per_unit": 100000000
    }
  },
  "smart_support": {
    "available": true,
    "enabled": true
  },
  "smart_status": {
    "passed": true
  },
  "ata_smart_data": {
    "offline_data_collection": {
      "status": {
        "value": 0,
        "string": "was never started"
      },
      "completion_seconds": 0
    },
    "self_test": {
      "status": {
        "value": 0,
        "string": "completed without error",
        "passed": true
      },
      "polling_minutes": {
        "short": 1,
        "extended": 48,
        "conveyance": 2
      }
    },
    "capabilities": {
      "values": [
        125,
        3
      ],
      "exec_offline_immediate_supported": true,
      "offline_is_aborted_upon_new_cmd": true,
      "offline_surface_scan_supported": true,
      "self_tests_supported": true,
      "conveyance_self_test_supported": true,
      "selective_self_test_supported": true,
      "attribute_autosave_enabled": true,
      "error_logging_supported": true,
      "gp_logging_supported": true
    }
  },
  "ata_sct_capabilities": {
    "value": 37,
    "error_recovery_control_supported": false,
    "feature_control_supported": false,
    "data_table_supported": true
  },
  "ata_smart_attributes": {
    "revision": 10,
    "table": [
      {
        "id": 1,
        "name": "Raw_Read_Error_Rate",
        "value": 95,
        "worst": 95,
        "thresh": 50,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 189861978,
          "string": "0/189861978"
        }
      },
      {
        "id": 5,
        "name": "Retired_Block_Count",
        "value": 100,
        "worst": 100,
        "thresh": 3,
        "when_failed": "",
        "flags": {
          "value": 51,
          "string": "PO--CK ",
          "prefailure": true,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 0,
          "string": "0"
        }
      },
      {
        "id": 9,
        "name": "Power_On_Hours_and_Msec",
        "value": 60,
        "worst": 60,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 683071598791665,
          "string": "35825h+02m+39.040s"
        }
      },
      {
        "id": 12,
        "name": "Power_Cycle_Count",
        "value": 100,
        "worst": 100,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 61,
          "string": "61"
        }
      },
      {
        "id": 171,
        "name": "Program_Fail_Count",
        "value": 100,
        "worst": 100,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 10,
          "string": "-O-R-- ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": true,
          "event_count": false,
          "auto_keep": false
        },
        "raw": {
          "value": 0,
          "string": "0"
        }
      },
      {
        "id": 172,
        "name": "Erase_Fail_Count",
        "value": 100,
        "worst": 100,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 0,
          "string": "0"
        }
      },
      {
        "id": 174,
        "name": "Unexpect_Power_Loss_Ct",
        "value": 0,
        "worst": 0,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 48,
          "string": "----CK ",
          "prefailure": false,
          "updated_online": false,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 31,
          "string": "31"
        }
      },
      {
        "id": 177,
        "name": "Wear_Range_Delta",
        "value": 0,
        "worst": 0,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 0,
          "string": "------ ",
          "prefailure": false,
          "updated_online": false,
          "performance": false,
          "error_rate": false,
          "event_count": false,
          "auto_keep": false
        },
        "raw": {
          "value": 1,
          "string": "1"
        }
      },
      {
        "id": 181,
        "name": "Program_Fail_Count",
        "value": 100,
        "worst": 100,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 10,
          "string": "-O-R-- ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": true,
          "event_count": false,
          "auto_keep": false
        },
        "raw": {
          "value": 0,
          "string": "0"
        }
      },
      {
        "id": 182,
        "name": "Erase_Fail_Count",
        "value": 100,
        "worst": 100,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 0,
          "string": "0"
        }
      },
      {
        "id": 187,
        "name": "Reported_Uncorrect",
        "value": 100,
        "worst": 100,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 18,
          "string": "-O--C- ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": false
        },
        "raw": {
          "value": 0,
          "string": "0"
        }
      },
      {
        "id": 189,
        "name": "Airflow_Temperature_Cel",
        "value": 29,
        "worst": 56,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 0,
          "string": "------ ",
          "prefailure": false,
          "updated_online": false,
          "performance": false,
          "error_rate": false,
          "event_count": false,
          "auto_keep": false
        },
        "raw": {
          "value": 77313081373,
          "string": "29 (Min/Max 18/56)"
        }
      },
      {
        "id": 194,
        "name": "Temperature_Celsius",
        "value": 29,
        "worst": 56,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 34,
          "string": "-O---K ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": false,
          "auto_keep": true
        },
        "raw": {
          "value": 77313081373,
          "string": "29 (Min/Max 18/56)"
        }
      },
      {
        "id": 195,
        "name": "ECC_Uncorr_Error_Count",
        "value": 120,
        "worst": 120,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 28,
          "string": "--SRC- ",
          "prefailure": false,
          "updated_online": false,
          "performance": true,
          "error_rate": true,
          "event_count": true,
          "auto_keep": false
        },
        "raw": {
          "value": 189861978,
          "string": "0/189861978"
        }
      },
      {
        "id": 196,
        "name": "Reallocated_Event_Count",
        "value": 100,
        "worst": 100,
        "thresh": 3,
        "when_failed": "",
        "flags": {
          "value": 51,
          "string": "PO--CK ",
          "prefailure": true,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 0,
          "string": "0"
        }
      },
      {
        "id": 201,
        "name": "Unc_Soft_Read_Err_Rate",
        "value": 120,
        "worst": 120,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 28,
          "string": "--SRC- ",
          "prefailure": false,
          "updated_online": false,
          "performance": true,
          "error_rate": true,
          "event_count": true,
          "auto_keep": false
        },
        "raw": {
          "value": 189861978,
          "string": "0/189861978"
        }
      },
      {
        "id": 204,
        "name": "Soft_ECC_Correct_Rate",
        "value": 120,
        "worst": 120,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 28,
          "string": "--SRC- ",
          "prefailure": false,
          "updated_online": false,
          "performance": true,
          "error_rate": true,
          "event_count": true,
          "auto_keep": false
        },
        "raw": {
          "value": 189861978,
          "string": "0/189861978"
        }
      },
      {
        "id": 230,
        "name": "Life_Curve_Status",
        "value": 100,
        "worst": 100,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 19,
          "string": "PO--C- ",
          "prefailure": true,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": false
        },
        "raw": {
          "value": 100,
          "string": "100"
        }
      },
      {
        "id": 231,
        "name": "SSD_Life_Left",
        "value": 95,
        "worst": 95,
        "thresh": 11,
        "when_failed": "",
        "flags": {
          "value": 0,
          "string": "------ ",
          "prefailure": false,
          "updated_online": false,
          "performance": false,
          "error_rate": false,
          "event_count": false,
          "auto_keep": false
        },
        "raw": {
          "value": 4294967296,
          "string": "4294967296"
        }
      },
      {
        "id": 233,
        "name": "SandForce_Internal",
        "value": 0,
        "worst": 0,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 51384,
          "string": "51384"
        }
      },
      {
        "id": 234,
        "name": "SandForce_Internal",
        "value": 0,
        "worst": 0,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 20878,
          "string": "20878"
        }
      },
      {
        "id": 241,
        "name": "Lifetime_Writes_GiB",
        "value": 0,
        "worst": 0,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 20878,
          "string": "20878"
        }
      },
      {
        "id": 242,
        "name": "Lifetime_Reads_GiB",
        "value": 0,
        "worst": 0,
        "thresh": 0,
        "when_failed": "",
        "flags": {
          "value": 50,
          "string": "-O--CK ",
          "prefailure": false,
          "updated_online": true,
          "performance": false,
          "error_rate": false,
          "event_count": true,
          "auto_keep": true
        },
        "raw": {
          "value": 40443,
          "string": "40443"
        }
      },
      {
        "id": 244,
        "name": "Unknown_Attribute",
        "value": 92,
        "worst": 92,
        "thresh": 10,
        "when_failed": "",
        "flags": {
          "value": 0,
          "string": "------ ",
          "prefailure": false,
          "updated_online": false,
          "performance": false,
          "error_rate": false,
          "event_count": false,
          "auto_keep": false
        },
        "raw": {
          "value": 15925491,
          "string": "15925491"
        }
      }
    ]
  },
  "spare_available": {
    "current_percent": 100,
    "threshold_percent": 3
  },
  "power_on_time": {
    "hours": 35825,
    "minutes": 2
  },
  "power_cycle_count": 61,
  "endurance_used": {
    "current_percent": 5
  },
  "temperature": {
    "current": 29
  },
  "ata_smart_self_test_log": {
    "standard": {
      "revision": 1,
      "count": 0
    }
  },
  "ata_smart_selective_self_test_log": {
    "revision": 1,
    "table": [
      {
        "lba_min": 0,
        "lba_max": 0,
        "status": {
          "value": 0,
          "string": "Not_testing"
        }
      },
      {
        "lba_min": 0,
        "lba_max": 0,
        "status": {
          "value": 0,
          "string": "Not_testing"
        }
      },
      {
        "lba_min": 0,
        "lba_max": 0,
        "status": {
          "value": 0,
          "string": "Not_testing"
        }
      },
      {
        "lba_min": 0,
        "lba_max": 0,
        "status": {
          "value": 0,
          "string": "Not_testing"
        }
      },
      {
        "lba_min": 0,
        "lba_max": 0,
        "status": {
          "value": 0,
          "string": "Not_testing"
        }
      }
    ],
    "flags": {
      "value": 0,
      "remainder_scan_enabled": false
    },
    "power_up_scan_resume_minutes": 0
  }
}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetSMARTInfo("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if info.Device.Name != "/dev/sda" {
		t.Errorf("Expected device name /dev/sda, got %s", info.Device.Name)
	}

	if info.ModelName != "KINGSTON SV300S37A240G" {
		t.Errorf("Expected model KINGSTON SV300S37A240G, got %s", info.ModelName)
	}

	if !info.SmartStatus.Passed {
		t.Error("Expected SMART status passed")
	}

	if info.Smartctl == nil || len(info.Smartctl.Messages) != 1 {
		t.Errorf("Expected 1 message, got %v", info.Smartctl)
	}

	if info.Smartctl.Messages[0].String != "Test informational message" {
		t.Errorf("Expected message 'Test informational message', got '%s'", info.Smartctl.Messages[0].String)
	}

	if info.Smartctl.Messages[0].Severity != "info" {
		t.Errorf("Expected severity 'info', got '%s'", info.Smartctl.Messages[0].Severity)
	}
}

func TestGetSMARTInfoExitError(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"smart_status": {"passed": false},
		"smartctl": {
			"messages": [
				{"string": "Test error message", "severity": "error"}
			]
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {
				output: []byte(mockJSON),
				err:    &exec.ExitError{Stderr: []byte("")},
			},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetSMARTInfo("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if info.SmartStatus.Passed {
		t.Error("Expected SMART status failed")
	}

	if info.Smartctl == nil || len(info.Smartctl.Messages) != 1 {
		t.Errorf("Expected 1 message, got %v", info.Smartctl)
	}

	if info.Smartctl.Messages[0].String != "Test error message" {
		t.Errorf("Expected message 'Test error message', got '%s'", info.Smartctl.Messages[0].String)
	}

	if info.Smartctl.Messages[0].Severity != "error" {
		t.Errorf("Expected severity 'error', got '%s'", info.Smartctl.Messages[0].Severity)
	}
}

func TestCheckHealth(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -H /dev/sda": {output: []byte("SMART overall-health self-assessment test result: PASSED")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	healthy, err := client.CheckHealth("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !healthy {
		t.Error("Expected device to be healthy")
	}
}

func TestCheckHealthFailed(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -H /dev/sda": {output: []byte("SMART overall-health self-assessment test result: FAILED")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	healthy, err := client.CheckHealth("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if healthy {
		t.Error("Expected device to be unhealthy")
	}
}

func TestCheckHealthExitError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -H /dev/sda": {
				output: []byte("some output"),
				err:    &exec.ExitError{Stderr: []byte("SMART overall-health self-assessment test result: PASSED")},
			},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	healthy, err := client.CheckHealth("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !healthy {
		t.Error("Expected device to be healthy from stderr")
	}
}

func TestGetDeviceInfo(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"model_name": "Test Drive",
		"serial_number": "12345"
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -i -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetDeviceInfo("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if model, ok := info["model_name"].(string); !ok || model != "Test Drive" {
		t.Errorf("Expected model_name 'Test Drive', got %v", info["model_name"])
	}
}

func TestRunSelfTest(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -t short /dev/sda": {},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.RunSelfTest("/dev/sda", "short")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestRunSelfTestInvalidType(t *testing.T) {
	client := NewClientWithPath("/usr/sbin/smartctl")

	err := client.RunSelfTest("/dev/sda", "invalid")
	if err == nil {
		t.Error("Expected error for invalid test type")
	}
}

func TestRunSelfTestWithProgressInvalidType(t *testing.T) {
	client := NewClientWithPath("/usr/sbin/smartctl")

	ctx := context.Background()
	err := client.RunSelfTestWithProgress(ctx, "/dev/sda", "invalid", nil)
	if err == nil {
		t.Error("Expected error for invalid test type")
	}
}

func TestRunSelfTestWithProgress(t *testing.T) {
	// Mock SMART info with ATA device supporting self-tests and completed status
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"ata_smart_data": {
			"capabilities": {
				"exec_offline_immediate_supported": true
			},
			"self_test": {
				"status": "completed",
				"polling_minutes": {
					"short": 2
				}
			}
		}
	}`

	mockCapabilitiesJSON := `{
		"ata_smart_data": {
			"capabilities": {
				"exec_offline_immediate_supported": true,
				"self_tests_supported": true
			},
			"self_test": {
				"polling_minutes": {
					"short": 2
				}
			}
		}
	}`

	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda":    {output: []byte(mockJSON)},
			"/usr/sbin/smartctl -c -j /dev/sda":    {output: []byte(mockCapabilitiesJSON)},
			"/usr/sbin/smartctl -t short /dev/sda": {},
		},
	}

	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var progressCalled bool
	var finalProgress int
	var finalStatus string

	callback := func(progress int, status string) {
		progressCalled = true
		finalProgress = progress
		finalStatus = status
	}

	err := client.RunSelfTestWithProgress(ctx, "/dev/sda", "short", callback)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !progressCalled {
		t.Error("Expected progress callback to be called")
	}

	if finalProgress != 100 {
		t.Errorf("Expected final progress 100, got %d", finalProgress)
	}

	if finalStatus != "Self-test completed" {
		t.Errorf("Expected final status 'Self-test completed', got '%s'", finalStatus)
	}
}

func TestGetAvailableSelfTestsATA(t *testing.T) {
	mockJSON := `{
		"ata_smart_data": {
			"capabilities": {
				"exec_offline_immediate_supported": true,
				"self_tests_supported": true,
				"conveyance_self_test_supported": true
			}
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetAvailableSelfTests("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expected := []string{"short", "long", "conveyance", "offline"}
	if len(info.Available) != len(expected) {
		t.Errorf("Expected %d tests, got %d", len(expected), len(info.Available))
	}

	for i, test := range expected {
		if i >= len(info.Available) || info.Available[i] != test {
			t.Errorf("Expected test %s at position %d, got %v", test, i, info.Available)
		}
	}
}

func TestGetAvailableSelfTestsATANoCapabilities(t *testing.T) {
	mockJSON := `{
		"ata_smart_data": {}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetAvailableSelfTests("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(info.Available) != 0 {
		t.Errorf("Expected no tests, got %v", info.Available)
	}
}

func TestGetAvailableSelfTestsNVMe(t *testing.T) {
	mockJSON := `{
		"nvme_controller_capabilities": {
			"self_test": true
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetAvailableSelfTests("/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expected := []string{"short"}
	if len(info.Available) != len(expected) || info.Available[0] != expected[0] {
		t.Errorf("Expected %v, got %v", expected, info.Available)
	}
}

func TestGetAvailableSelfTestsNVMeNoSupport(t *testing.T) {
	mockJSON := `{
		"nvme_controller_capabilities": {
			"self_test": false
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	info, err := client.GetAvailableSelfTests("/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(info.Available) != 0 {
		t.Errorf("Expected no tests, got %v", info.Available)
	}
}

func TestGetAvailableSelfTestsError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -c -j /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	_, err := client.GetAvailableSelfTests("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestIsSMARTSupportedATA(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"},
		"ata_smart_data": {
			"capabilities": {
				"exec_offline_immediate_supported": true
			}
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	supportInfo, err := client.IsSMARTSupported("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !supportInfo.Supported {
		t.Error("Expected SMART to be supported for ATA device")
	}

	if !supportInfo.Enabled {
		t.Error("Expected SMART to be enabled for ATA device")
	}
}

func TestIsSMARTSupportedNVMe(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/nvme0n1", "type": "nvme"},
		"nvme_smart_health_information_log": {
			"temperature": 35
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	supportInfo, err := client.IsSMARTSupported("/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !supportInfo.Supported {
		t.Error("Expected SMART to be supported for NVMe device")
	}

	if !supportInfo.Enabled {
		t.Error("Expected SMART to be enabled for NVMe device")
	}
}

func TestIsSMARTSupportedNVMeWithSmartSupport(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/nvme0n1", "type": "nvme"},
		"smart_support": {
			"available": true,
			"enabled": false
		}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/nvme0n1": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	supportInfo, err := client.IsSMARTSupported("/dev/nvme0n1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !supportInfo.Supported {
		t.Error("Expected SMART to be supported for NVMe device")
	}

	if supportInfo.Enabled {
		t.Error("Expected SMART to be disabled for NVMe device")
	}
}

func TestIsSMARTSupportedNotSupported(t *testing.T) {
	mockJSON := `{
		"device": {"name": "/dev/sda", "type": "ata"}
	}`
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {output: []byte(mockJSON)},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	supportInfo, err := client.IsSMARTSupported("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if supportInfo.Supported {
		t.Error("Expected SMART to not be supported")
	}

	if supportInfo.Enabled {
		t.Error("Expected SMART to not be enabled")
	}
}

func TestIsSMARTSupportedError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -a -j /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	_, err := client.IsSMARTSupported("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestEnableSMART(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -s on /dev/sda": {},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.EnableSMART("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestEnableSMARTError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -s on /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.EnableSMART("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestDisableSMART(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -s off /dev/sda": {},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.DisableSMART("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestDisableSMARTError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -s off /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.DisableSMART("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestAbortSelfTest(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -X /dev/sda": {},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.AbortSelfTest("/dev/sda")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestAbortSelfTestError(t *testing.T) {
	commander := &mockCommander{
		cmds: map[string]*mockCmd{
			"/usr/sbin/smartctl -X /dev/sda": {err: errors.New("command failed")},
		},
	}
	client := NewClientWithCommander("/usr/sbin/smartctl", commander)

	err := client.AbortSelfTest("/dev/sda")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}
