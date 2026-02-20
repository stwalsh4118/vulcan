package firecracker

import (
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Clear all FC env vars to ensure defaults.
	for _, env := range []string{
		envKernelPath, envRootfsDir, envBin,
		envCNIConfigDir, envCNIBinDir, envVsockPort, envJailer,
	} {
		t.Setenv(env, "")
	}

	cfg := LoadConfig()

	if cfg.VsockPort != DefaultVsockPort {
		t.Errorf("VsockPort = %d, want %d", cfg.VsockPort, DefaultVsockPort)
	}
	if cfg.CIDBase != MinCID {
		t.Errorf("CIDBase = %d, want %d", cfg.CIDBase, MinCID)
	}
	if cfg.DefaultVCPUs != DefaultVCPUs {
		t.Errorf("DefaultVCPUs = %d, want %d", cfg.DefaultVCPUs, DefaultVCPUs)
	}
	if cfg.DefaultMemMB != DefaultMemMB {
		t.Errorf("DefaultMemMB = %d, want %d", cfg.DefaultMemMB, DefaultMemMB)
	}
	if cfg.JailerEnabled {
		t.Error("JailerEnabled should be false by default")
	}
	if cfg.KernelPath != "" {
		t.Errorf("KernelPath = %q, want empty", cfg.KernelPath)
	}
}

func TestLoadConfigEnvOverrides(t *testing.T) {
	t.Setenv(envKernelPath, "/opt/vmlinux")
	t.Setenv(envRootfsDir, "/opt/rootfs")
	t.Setenv(envBin, "/usr/bin/firecracker")
	t.Setenv(envCNIConfigDir, "/etc/cni/conf.d")
	t.Setenv(envCNIBinDir, "/opt/cni/bin")
	t.Setenv(envVsockPort, "2048")
	t.Setenv(envJailer, "true")

	cfg := LoadConfig()

	if cfg.KernelPath != "/opt/vmlinux" {
		t.Errorf("KernelPath = %q, want /opt/vmlinux", cfg.KernelPath)
	}
	if cfg.RootfsDir != "/opt/rootfs" {
		t.Errorf("RootfsDir = %q, want /opt/rootfs", cfg.RootfsDir)
	}
	if cfg.FirecrackerBin != "/usr/bin/firecracker" {
		t.Errorf("FirecrackerBin = %q, want /usr/bin/firecracker", cfg.FirecrackerBin)
	}
	if cfg.CNIConfigDir != "/etc/cni/conf.d" {
		t.Errorf("CNIConfigDir = %q, want /etc/cni/conf.d", cfg.CNIConfigDir)
	}
	if cfg.CNIBinDir != "/opt/cni/bin" {
		t.Errorf("CNIBinDir = %q, want /opt/cni/bin", cfg.CNIBinDir)
	}
	if cfg.VsockPort != 2048 {
		t.Errorf("VsockPort = %d, want 2048", cfg.VsockPort)
	}
	if !cfg.JailerEnabled {
		t.Error("JailerEnabled should be true when VULCAN_FC_JAILER=true")
	}
}

func TestLoadConfigJailerVariants(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv(envJailer, tt.value)
			cfg := LoadConfig()
			if cfg.JailerEnabled != tt.want {
				t.Errorf("JailerEnabled = %v for %q, want %v", cfg.JailerEnabled, tt.value, tt.want)
			}
		})
	}
}

func TestLoadConfigInvalidVsockPort(t *testing.T) {
	t.Setenv(envVsockPort, "not-a-number")
	cfg := LoadConfig()
	if cfg.VsockPort != DefaultVsockPort {
		t.Errorf("VsockPort = %d, want default %d for invalid input", cfg.VsockPort, DefaultVsockPort)
	}
}
