package firecracker

import (
	"os"
	"strconv"
	"strings"
)

// Environment variable names for Firecracker configuration.
const (
	envKernelPath   = "VULCAN_FC_KERNEL_PATH"
	envRootfsDir    = "VULCAN_FC_ROOTFS_DIR"
	envBin          = "VULCAN_FC_BIN"
	envCNIConfigDir = "VULCAN_FC_CNI_CONFIG_DIR"
	envCNIBinDir    = "VULCAN_FC_CNI_BIN_DIR"
	envVsockPort       = "VULCAN_FC_VSOCK_PORT"
	envMaxConcurrent   = "VULCAN_FC_MAX_CONCURRENT_VMS"
	envJailer          = "VULCAN_FC_JAILER"
)

// Config holds configuration for the Firecracker microVM backend.
type Config struct {
	// KernelPath is the path to the Firecracker-compatible kernel image.
	KernelPath string

	// RootfsDir is the directory containing runtime-specific rootfs images.
	RootfsDir string

	// FirecrackerBin is the path to the Firecracker binary.
	FirecrackerBin string

	// CNIConfigDir is the path to CNI configuration directory.
	CNIConfigDir string

	// CNIBinDir is the path to CNI plugin binaries.
	CNIBinDir string

	// VsockPort is the guest agent vsock port.
	VsockPort uint32

	// CIDBase is the starting context ID for vsock.
	CIDBase uint32

	// JailerEnabled controls whether the Firecracker jailer is used.
	JailerEnabled bool

	// DefaultVCPUs is the default vCPU count per microVM.
	DefaultVCPUs int

	// DefaultMemMB is the default memory in MB per microVM.
	DefaultMemMB int

	// MaxConcurrentVMs is the maximum number of concurrent microVMs.
	MaxConcurrentVMs int
}

// LoadConfig reads Firecracker configuration from environment variables,
// applying sensible defaults for values not set.
func LoadConfig() Config {
	cfg := Config{
		VsockPort:        DefaultVsockPort,
		CIDBase:          MinCID,
		DefaultVCPUs:     DefaultVCPUs,
		DefaultMemMB:     DefaultMemMB,
		MaxConcurrentVMs: MaxConcurrentVMs,
	}

	if v := os.Getenv(envKernelPath); v != "" {
		cfg.KernelPath = v
	}
	if v := os.Getenv(envRootfsDir); v != "" {
		cfg.RootfsDir = v
	}
	if v := os.Getenv(envBin); v != "" {
		cfg.FirecrackerBin = v
	}
	if v := os.Getenv(envCNIConfigDir); v != "" {
		cfg.CNIConfigDir = v
	}
	if v := os.Getenv(envCNIBinDir); v != "" {
		cfg.CNIBinDir = v
	}
	if v := os.Getenv(envVsockPort); v != "" {
		if port, err := strconv.ParseUint(v, 10, 32); err == nil {
			cfg.VsockPort = uint32(port)
		}
	}
	if v := os.Getenv(envMaxConcurrent); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxConcurrentVMs = n
		}
	}
	if v := os.Getenv(envJailer); v != "" {
		cfg.JailerEnabled = strings.EqualFold(v, "true") || v == "1"
	}

	return cfg
}
