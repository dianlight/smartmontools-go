package smartmontools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSmartctlSearchPaths_NotEmpty verifies the fallback search list is populated.
func TestSmartctlSearchPaths_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, smartctlSearchPaths, "smartctlSearchPaths must contain at least one entry")
}

// TestSmartctlSearchPaths_ContainsPlatformPaths verifies that the most common
// platform-specific installation locations are represented.
func TestSmartctlSearchPaths_ContainsPlatformPaths(t *testing.T) {
	expected := []string{
		"/usr/sbin/smartctl",                   // Standard Linux
		"/usr/local/sbin/smartctl",             // FreeBSD / TrueNAS CORE
		"/opt/homebrew/bin/smartctl",           // macOS Homebrew Apple Silicon
		"/usr/local/bin/smartctl",              // macOS Homebrew Intel
		"/usr/syno/bin/smartctl",               // Synology DSM
		"/run/current-system/sw/sbin/smartctl", // NixOS
	}
	for _, want := range expected {
		assert.Contains(t, smartctlSearchPaths, want,
			"smartctlSearchPaths should include %q", want)
	}
}

// TestResolveSmartctlPath_SearchPathFallback verifies that resolveSmartctlPath
// finds a binary through the fallback search list when PATH is empty.
func TestResolveSmartctlPath_SearchPathFallback(t *testing.T) {
	// Create a temporary executable to act as a fake smartctl.
	tmpDir := t.TempDir()
	fakeSmartctl := filepath.Join(tmpDir, "smartctl")
	err := os.WriteFile(fakeSmartctl, []byte("#!/bin/sh\necho fake"), 0o755)
	require.NoError(t, err)

	// Temporarily prepend the fake path to the search list.
	orig := smartctlSearchPaths
	t.Cleanup(func() { smartctlSearchPaths = orig })
	smartctlSearchPaths = append([]string{fakeSmartctl}, orig...)

	// Clear PATH so exec.LookPath cannot find anything.
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "")

	got, err := resolveSmartctlPath()
	require.NoError(t, err)
	assert.Equal(t, fakeSmartctl, got)
}

// TestResolveSmartctlPath_NotFound verifies that resolveSmartctlPath returns an
// error with actionable install instructions when smartctl cannot be located.
func TestResolveSmartctlPath_NotFound(t *testing.T) {
	// Empty the search list and clear PATH.
	orig := smartctlSearchPaths
	t.Cleanup(func() { smartctlSearchPaths = orig })
	smartctlSearchPaths = []string{}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "")

	_, err := resolveSmartctlPath()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "smartctl not found"),
		"error should mention 'smartctl not found', got: %v", err)
}

// TestResolveSmartctlPath_SkipsNonExecutable verifies that files without the
// executable bit are skipped and the function continues to the next candidate.
func TestResolveSmartctlPath_SkipsNonExecutable(t *testing.T) {
	tmpDir := t.TempDir()

	// Non-executable file — should be skipped.
	nonExec := filepath.Join(tmpDir, "smartctl-noexec")
	require.NoError(t, os.WriteFile(nonExec, []byte("#!/bin/sh"), 0o644))

	// Executable file — should be returned.
	execFile := filepath.Join(tmpDir, "smartctl-exec")
	require.NoError(t, os.WriteFile(execFile, []byte("#!/bin/sh"), 0o755))

	orig := smartctlSearchPaths
	t.Cleanup(func() { smartctlSearchPaths = orig })
	smartctlSearchPaths = []string{nonExec, execFile}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "")

	got, err := resolveSmartctlPath()
	require.NoError(t, err)
	assert.Equal(t, execFile, got, "non-executable candidate should be skipped")
}

// TestResolveSmartctlPath_SkipsDirectories verifies that directory entries in
// the search list are silently skipped.
func TestResolveSmartctlPath_SkipsDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// A directory that happens to be named "smartctl" — should be skipped.
	dirPath := filepath.Join(tmpDir, "smartctl-dir")
	require.NoError(t, os.Mkdir(dirPath, 0o755))

	// A real executable that should be found after skipping the directory.
	execFile := filepath.Join(tmpDir, "smartctl-real")
	require.NoError(t, os.WriteFile(execFile, []byte("#!/bin/sh"), 0o755))

	orig := smartctlSearchPaths
	t.Cleanup(func() { smartctlSearchPaths = orig })
	smartctlSearchPaths = []string{dirPath, execFile}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", "")

	got, err := resolveSmartctlPath()
	require.NoError(t, err)
	assert.Equal(t, execFile, got, "directory entry should be skipped")
}
