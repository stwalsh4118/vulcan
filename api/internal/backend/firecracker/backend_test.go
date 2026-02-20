package firecracker

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/model"
)

func TestCapabilities(t *testing.T) {
	b := &Backend{
		cfg: Config{
			DefaultVCPUs:     DefaultVCPUs,
			DefaultMemMB:     DefaultMemMB,
			MaxConcurrentVMs: MaxConcurrentVMs,
		},
	}

	caps := b.Capabilities()

	if caps.Name != BackendName {
		t.Errorf("Name = %q, want %q", caps.Name, BackendName)
	}

	if len(caps.SupportedRuntimes) != len(SupportedRuntimes) {
		t.Errorf("SupportedRuntimes length = %d, want %d",
			len(caps.SupportedRuntimes), len(SupportedRuntimes))
	}
	for i, rt := range caps.SupportedRuntimes {
		if rt != SupportedRuntimes[i] {
			t.Errorf("SupportedRuntimes[%d] = %q, want %q", i, rt, SupportedRuntimes[i])
		}
	}

	if len(caps.SupportedIsolations) != 1 || caps.SupportedIsolations[0] != model.IsolationMicroVM {
		t.Errorf("SupportedIsolations = %v, want [%q]", caps.SupportedIsolations, model.IsolationMicroVM)
	}

	if caps.MaxConcurrency != MaxConcurrentVMs {
		t.Errorf("MaxConcurrency = %d, want %d", caps.MaxConcurrency, MaxConcurrentVMs)
	}
}

func TestCapabilitiesCustomConcurrency(t *testing.T) {
	customMax := 25
	b := &Backend{
		cfg: Config{
			MaxConcurrentVMs: customMax,
		},
	}

	caps := b.Capabilities()
	if caps.MaxConcurrency != customMax {
		t.Errorf("MaxConcurrency = %d, want %d", caps.MaxConcurrency, customMax)
	}
}

func TestCIDAllocateAndRelease(t *testing.T) {
	b := &Backend{
		cfg:      Config{MaxConcurrentVMs: MaxConcurrentVMs},
		cidNext:  MinCID,
		cidInUse: make(map[uint32]bool),
	}

	// Allocate first CID.
	cid1, err := b.allocateCID()
	if err != nil {
		t.Fatalf("first allocate: %v", err)
	}
	if cid1 < MinCID {
		t.Errorf("cid1 = %d, want >= %d", cid1, MinCID)
	}

	// Allocate second CID — should be different.
	cid2, err := b.allocateCID()
	if err != nil {
		t.Fatalf("second allocate: %v", err)
	}
	if cid2 == cid1 {
		t.Errorf("cid2 should differ from cid1 (%d)", cid1)
	}

	// Release first CID.
	b.releaseCID(cid1)

	// Verify it's no longer in use.
	b.cidMu.Lock()
	if b.cidInUse[cid1] {
		t.Error("cid1 should be released")
	}
	b.cidMu.Unlock()
}

func TestCIDAllocateConcurrent(t *testing.T) {
	b := &Backend{
		cfg:      Config{MaxConcurrentVMs: MaxConcurrentVMs},
		cidNext:  MinCID,
		cidInUse: make(map[uint32]bool),
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	cids := make(chan uint32, numGoroutines)

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cid, err := b.allocateCID()
			if err != nil {
				t.Errorf("allocate: %v", err)
				return
			}
			cids <- cid
		}()
	}

	wg.Wait()
	close(cids)

	// Verify all CIDs are unique.
	seen := make(map[uint32]bool)
	for cid := range cids {
		if seen[cid] {
			t.Errorf("duplicate CID: %d", cid)
		}
		seen[cid] = true
	}

	if len(seen) != numGoroutines {
		t.Errorf("allocated %d CIDs, want %d", len(seen), numGoroutines)
	}
}

func TestCIDAllocateExhaustion(t *testing.T) {
	b := &Backend{
		cfg:      Config{MaxConcurrentVMs: MaxConcurrentVMs},
		cidNext:  MinCID,
		cidInUse: make(map[uint32]bool),
	}

	// Pre-fill the scan window (MaxConcurrentVMs+10 slots ahead of cidNext).
	scanRange := uint32(MaxConcurrentVMs + 10)
	for i := range scanRange {
		b.cidInUse[MinCID+i] = true
	}

	// Should fail — all CIDs in the scan window are taken.
	_, err := b.allocateCID()
	if err == nil {
		t.Fatal("expected error when all CIDs exhausted")
	}

	// Release one and allocate again.
	b.releaseCID(MinCID)
	cid, err := b.allocateCID()
	if err != nil {
		t.Fatalf("should be able to allocate after release: %v", err)
	}
	if cid != MinCID {
		t.Errorf("expected to reuse released CID %d, got %d", MinCID, cid)
	}
}

func TestCleanupNonexistent(t *testing.T) {
	b := &Backend{
		activeVMs: make(map[string]*vmState),
		logger:    testLogger(),
	}

	// Cleanup for a workload that doesn't exist should be a no-op.
	err := b.Cleanup(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("Cleanup nonexistent: %v", err)
	}
}

func TestBackendImplementsInterface(t *testing.T) {
	// Compile-time check that Backend implements backend.Backend.
	var _ backend.Backend = (*Backend)(nil)
}

func TestCopyRootfs(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create a source rootfs file.
	srcPath := filepath.Join(srcDir, "test.ext4")
	content := []byte("fake rootfs content for testing")
	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatalf("write source rootfs: %v", err)
	}

	dstPath := filepath.Join(dstDir, "copy.ext4")
	if err := copyRootfs(srcPath, dstPath); err != nil {
		t.Fatalf("copyRootfs: %v", err)
	}

	// Verify the copy exists and has correct content.
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read copy: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copy content = %q, want %q", string(got), string(content))
	}
}

func TestCopyRootfsMissing(t *testing.T) {
	dstDir := t.TempDir()
	dstPath := filepath.Join(dstDir, "copy.ext4")

	err := copyRootfs("/nonexistent/rootfs.ext4", dstPath)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestDefaultBootArgs(t *testing.T) {
	// Verify boot args contain expected components.
	expected := []string{
		"console=ttyS0",
		"reboot=k",
		"panic=1",
		"pci=off",
		"init=" + GuestAgentPath,
	}

	for _, arg := range expected {
		if !containsArg(DefaultBootArgs, arg) {
			t.Errorf("DefaultBootArgs missing %q: %s", arg, DefaultBootArgs)
		}
	}
}

func containsArg(args, arg string) bool {
	return slices.Contains(strings.Fields(args), arg)
}
