package firecracker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	fcsdk "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/sirupsen/logrus"

	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/model"
)

// Backend constants.
const (
	// BackendName is the name used when registering with the backend registry.
	BackendName = "firecracker"

	// DefaultBootArgs are the kernel boot arguments for Firecracker microVMs.
	DefaultBootArgs = "console=ttyS0 reboot=k panic=1 pci=off init=" + GuestAgentPath

	// vsockDeviceID is the device identifier used for vsock configuration.
	vsockDeviceID = "vsock0"

	// rootfsDriveID is the drive identifier for the root filesystem.
	rootfsDriveID = "rootfs"

	// vmSocketSuffix is appended to the workload ID for the VM socket.
	vmSocketSuffix = ".sock"

	// vsockSocketSuffix is appended for the vsock UDS path.
	vsockSocketSuffix = "_vsock.sock"

	// gracefulShutdownTimeout is the time allowed for graceful VM shutdown.
	gracefulShutdownTimeout = 3 * time.Second
)

// vmState tracks the state of an active microVM.
type vmState struct {
	machine   *fcsdk.Machine
	cid       uint32
	netConfig *NetworkConfig
	socketDir string // temp directory for socket files and rootfs copy
	started   bool   // true after machine.Start succeeds (guards activeVMs gauge)
}

// Backend implements the backend.Backend interface using Firecracker microVMs.
type Backend struct {
	cfg     Config
	netMgr  *NetworkManager
	logger  *slog.Logger

	mu       sync.Mutex
	activeVMs map[string]*vmState // workloadID → vmState

	cidMu    sync.Mutex
	cidNext  uint32
	cidInUse map[uint32]bool
}

// NewBackend creates a new Firecracker backend.
func NewBackend(cfg Config, logger *slog.Logger) (*Backend, error) {
	netMgr, err := NewNetworkManager(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("create network manager: %w", err)
	}

	return &Backend{
		cfg:       cfg,
		netMgr:    netMgr,
		logger:    logger,
		activeVMs: make(map[string]*vmState),
		cidNext:   cfg.CIDBase,
		cidInUse:  make(map[uint32]bool),
	}, nil
}

// Verify checks that CNI plugins are available.
func (b *Backend) Verify() error {
	return b.netMgr.Verify()
}

// Execute runs a workload inside a Firecracker microVM.
func (b *Backend) Execute(ctx context.Context, spec backend.WorkloadSpec) (backend.WorkloadResult, error) {
	start := time.Now()

	// 1. Select rootfs image.
	rootfsPath, err := RootfsPath(b.cfg.RootfsDir, spec.Runtime)
	if err != nil {
		return backend.WorkloadResult{}, fmt.Errorf("select rootfs: %w", err)
	}

	// 2. Allocate CID.
	cid, err := b.allocateCID()
	if err != nil {
		return backend.WorkloadResult{}, fmt.Errorf("allocate CID: %w", err)
	}

	// 3. Set up CNI networking.
	netCfg, err := b.netMgr.Setup(ctx, spec.ID)
	if err != nil {
		b.releaseCID(cid)
		return backend.WorkloadResult{}, fmt.Errorf("network setup: %w", err)
	}

	// 4. Create temporary directory for socket and rootfs copy.
	socketDir, err := os.MkdirTemp("", "vulcan-vm-"+spec.ID+"-")
	if err != nil {
		b.releaseCID(cid)
		b.cleanupResources(ctx, spec.ID, "")
		return backend.WorkloadResult{}, fmt.Errorf("create temp dir: %w", err)
	}

	// 5. Copy rootfs for this VM (copy-on-write when possible).
	vmRootfs := filepath.Join(socketDir, "rootfs.ext4")
	if err := copyRootfs(rootfsPath, vmRootfs); err != nil {
		b.releaseCID(cid)
		b.cleanupResources(ctx, spec.ID, socketDir)
		return backend.WorkloadResult{}, fmt.Errorf("copy rootfs: %w", err)
	}

	// 6. Configure VM.
	socketPath := filepath.Join(socketDir, spec.ID+vmSocketSuffix)
	vsockPath := filepath.Join(socketDir, spec.ID+vsockSocketSuffix)

	vcpus := int64(b.cfg.DefaultVCPUs)
	if spec.CPULimit > 0 {
		vcpus = int64(spec.CPULimit)
	}
	memMB := int64(b.cfg.DefaultMemMB)
	if spec.MemLimitMB > 0 {
		memMB = int64(spec.MemLimitMB)
	}

	fcCfg := fcsdk.Config{
		SocketPath:      socketPath,
		KernelImagePath: b.cfg.KernelPath,
		KernelArgs:      DefaultBootArgs,
		Drives: []models.Drive{
			{
				DriveID:      fcsdk.String(rootfsDriveID),
				PathOnHost:   fcsdk.String(vmRootfs),
				IsRootDevice: fcsdk.Bool(true),
				IsReadOnly:   fcsdk.Bool(false),
			},
		},
		NetworkInterfaces: fcsdk.NetworkInterfaces{
			{
				StaticConfiguration: &fcsdk.StaticNetworkConfiguration{
					MacAddress:  netCfg.MACAddress,
					HostDevName: netCfg.TAPDevice,
				},
			},
		},
		VsockDevices: []fcsdk.VsockDevice{
			{
				ID:   vsockDeviceID,
				Path: vsockPath,
				CID:  cid,
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  fcsdk.Int64(vcpus),
			MemSizeMib: fcsdk.Int64(memMB),
			Smt:        fcsdk.Bool(false),
		},
		NetNS: netCfg.NamespacePath,
		VMID:  spec.ID,
	}

	// Create a logrus logger that discards output (we use slog).
	fcLogger := logrus.New()
	fcLogger.SetOutput(io.Discard)

	// Use the configured Firecracker binary path.
	fcCmd := fcsdk.VMCommandBuilder{}.
		WithBin(b.cfg.FirecrackerBin).
		WithSocketPath(socketPath).
		Build(ctx)

	machine, err := fcsdk.NewMachine(ctx, fcCfg,
		fcsdk.WithLogger(logrus.NewEntry(fcLogger)),
		fcsdk.WithProcessRunner(fcCmd),
	)
	if err != nil {
		b.releaseCID(cid)
		b.cleanupResources(ctx, spec.ID, socketDir)
		return backend.WorkloadResult{}, fmt.Errorf("create machine: %w", err)
	}

	// Track the VM.
	state := &vmState{
		machine:   machine,
		cid:       cid,
		netConfig: netCfg,
		socketDir: socketDir,
	}
	b.mu.Lock()
	b.activeVMs[spec.ID] = state
	b.mu.Unlock()

	// Ensure cleanup on all exit paths.
	defer func() {
		b.stopAndCleanup(ctx, spec.ID, state)
	}()

	// 7. Start VM.
	bootStart := time.Now()
	if err := machine.Start(ctx); err != nil {
		workloadsTotal.WithLabelValues(spec.Runtime, statusFailed).Inc()
		return backend.WorkloadResult{}, fmt.Errorf("start VM: %w", err)
	}
	state.started = true
	activeVMs.Inc()

	b.logger.Info("VM started",
		"workload_id", spec.ID,
		"runtime", spec.Runtime,
		"cid", cid,
		"vcpus", vcpus,
		"mem_mb", memMB,
	)

	// 8. Connect to guest agent via vsock.
	gc, err := DialGuest(ctx, vsockPath, b.cfg.VsockPort)
	vmBootDuration.Observe(time.Since(bootStart).Seconds())
	if err != nil {
		workloadsTotal.WithLabelValues(spec.Runtime, statusFailed).Inc()
		return backend.WorkloadResult{}, fmt.Errorf("connect to guest: %w", err)
	}
	defer gc.Close()

	// 9. Send workload and stream results.
	req := GuestRequest{
		Runtime:     spec.Runtime,
		Code:        spec.Code,
		CodeArchive: spec.CodeArchive,
		Input:       spec.Input,
		TimeoutS:    spec.TimeoutS,
	}

	vsockStart := time.Now()
	resp, err := gc.RunWorkload(req, spec.LogWriter)
	vsockWorkloadDuration.Observe(time.Since(vsockStart).Seconds())
	if err != nil {
		if ctx.Err() != nil {
			workloadsTotal.WithLabelValues(spec.Runtime, statusKilled).Inc()
		} else {
			workloadsTotal.WithLabelValues(spec.Runtime, statusFailed).Inc()
		}
		return backend.WorkloadResult{}, fmt.Errorf("run workload: %w", err)
	}

	workloadsTotal.WithLabelValues(spec.Runtime, statusCompleted).Inc()

	duration := time.Since(start)

	b.logger.Info("workload completed",
		"workload_id", spec.ID,
		"exit_code", resp.ExitCode,
		"duration_ms", duration.Milliseconds(),
	)

	return backend.WorkloadResult{
		ExitCode:   resp.ExitCode,
		Output:     []byte(resp.Output),
		Error:      resp.Error,
		DurationMS: int(duration.Milliseconds()),
		LogLines:   resp.LogLines,
	}, nil
}

// Capabilities reports what this backend supports.
func (b *Backend) Capabilities() backend.BackendCapabilities {
	return backend.BackendCapabilities{
		Name:                BackendName,
		SupportedRuntimes:   SupportedRuntimes,
		SupportedIsolations: []string{model.IsolationMicroVM},
		MaxConcurrency:      b.cfg.MaxConcurrentVMs,
	}
}

// Cleanup releases resources for a specific workload.
func (b *Backend) Cleanup(ctx context.Context, workloadID string) error {
	b.mu.Lock()
	state, exists := b.activeVMs[workloadID]
	if !exists {
		b.mu.Unlock()
		return nil
	}
	delete(b.activeVMs, workloadID)
	b.mu.Unlock()

	b.stopAndCleanup(ctx, workloadID, state)
	return nil
}

// Shutdown gracefully stops all active VMs and cleans up.
func (b *Backend) Shutdown(ctx context.Context) {
	b.mu.Lock()
	ids := make([]string, 0, len(b.activeVMs))
	for id := range b.activeVMs {
		ids = append(ids, id)
	}
	b.mu.Unlock()

	for _, id := range ids {
		if err := b.Cleanup(ctx, id); err != nil {
			b.logger.Error("shutdown cleanup failed", "workload_id", id, "error", err)
		}
	}

	b.netMgr.TeardownAll(ctx)
}

// stopAndCleanup stops a VM and cleans up all associated resources.
// It uses background contexts for cleanup operations to ensure they complete
// even if the caller's context has been cancelled.
func (b *Backend) stopAndCleanup(_ context.Context, workloadID string, state *vmState) {
	cleanupStart := time.Now()

	// Remove from active VMs if still present.
	b.mu.Lock()
	delete(b.activeVMs, workloadID)
	b.mu.Unlock()

	// Stop the VM.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	if err := state.machine.Shutdown(shutdownCtx); err != nil {
		b.logger.Debug("graceful shutdown failed, forcing stop", "workload_id", workloadID, "error", err)
		if stopErr := state.machine.StopVMM(); stopErr != nil {
			b.logger.Debug("StopVMM failed", "workload_id", workloadID, "error", stopErr)
		}
	}

	// Wait for the process to exit.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer waitCancel()
	if err := state.machine.Wait(waitCtx); err != nil {
		b.logger.Debug("failed to wait for VM exit", "workload_id", workloadID, "error", err)
	}

	if state.started {
		activeVMs.Dec()
	}

	// Release CID.
	b.releaseCID(state.cid)

	// Teardown networking with a fresh context (caller's may be cancelled).
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cleanupCancel()
	b.teardownNetwork(cleanupCtx, workloadID)

	// Clean up temp files.
	if state.socketDir != "" {
		os.RemoveAll(state.socketDir)
	}

	vmCleanupDuration.Observe(time.Since(cleanupStart).Seconds())
	b.logger.Debug("cleanup complete", "workload_id", workloadID)
}

// cleanupResources handles cleanup when VM creation fails before tracking.
func (b *Backend) cleanupResources(ctx context.Context, workloadID, socketDir string) {
	b.teardownNetwork(ctx, workloadID)
	if socketDir != "" {
		os.RemoveAll(socketDir)
	}
}

// teardownNetwork tears down networking for a VM, logging errors but not propagating them.
func (b *Backend) teardownNetwork(ctx context.Context, workloadID string) {
	if err := b.netMgr.Teardown(ctx, workloadID); err != nil {
		b.logger.Warn("network teardown failed", "workload_id", workloadID, "error", err)
	}
}

// allocateCID returns the next available vsock CID.
func (b *Backend) allocateCID() (uint32, error) {
	b.cidMu.Lock()
	defer b.cidMu.Unlock()

	// Try the next CID and scan forward if in use.
	scanRange := uint32(b.cfg.MaxConcurrentVMs + 10)
	for i := range scanRange {
		candidate := max(b.cidNext+i, MinCID)
		if !b.cidInUse[candidate] {
			b.cidInUse[candidate] = true
			b.cidNext = candidate + 1
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("no available CIDs (all %d slots in use)", len(b.cidInUse))
}

// releaseCID returns a CID to the pool.
func (b *Backend) releaseCID(cid uint32) {
	b.cidMu.Lock()
	defer b.cidMu.Unlock()
	delete(b.cidInUse, cid)
}

// copyRootfs creates a copy of the rootfs image for a VM.
// Uses cp --reflink=auto for copy-on-write when the filesystem supports it.
func copyRootfs(src, dst string) error {
	cmd := exec.Command("cp", "--reflink=auto", src, dst)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp %s %s: %s: %w", src, dst, string(output), err)
	}
	return nil
}
