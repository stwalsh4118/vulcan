package firecracker

import (
	"fmt"
	"path/filepath"
	"slices"
)

// Default vsock settings.
const (
	// DefaultVsockPort is the port the guest agent listens on inside the microVM.
	DefaultVsockPort uint32 = 1024

	// MinCID is the minimum context ID for vsock; CIDs 0-2 are reserved.
	MinCID uint32 = 3
)

// Default resource limits.
const (
	DefaultVCPUs = 1
	DefaultMemMB = 512
)

// SupportedRuntimes lists the runtimes available in pre-built rootfs images.
var SupportedRuntimes = []string{"go", "node", "python"}

// RootfsFilename is the format string for rootfs image filenames (e.g. "go.ext4").
const RootfsFilename = "%s.ext4"

// Guest paths.
const (
	// GuestWorkDir is the directory inside the microVM where workload code is extracted.
	GuestWorkDir = "/work"

	// GuestAgentPath is the path to the guest agent binary inside the rootfs.
	GuestAgentPath = "/usr/local/bin/vulcan-guest"
)

// MaxConcurrentVMs is the default maximum number of concurrent microVMs.
const MaxConcurrentVMs = 10

// RootfsPath returns the full path to the rootfs image for a given runtime.
func RootfsPath(rootfsDir, runtime string) (string, error) {
	if !isSupportedRuntime(runtime) {
		return "", fmt.Errorf("unsupported runtime %q: must be one of %v", runtime, SupportedRuntimes)
	}
	return filepath.Join(rootfsDir, fmt.Sprintf(RootfsFilename, runtime)), nil
}

func isSupportedRuntime(runtime string) bool {
	return slices.Contains(SupportedRuntimes, runtime)
}
