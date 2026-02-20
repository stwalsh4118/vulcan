# Firecracker Go SDK Guide for Task 4-6

**Date**: 2026-02-19
**Package**: `github.com/firecracker-microvm/firecracker-go-sdk`
**Docs**: https://pkg.go.dev/github.com/firecracker-microvm/firecracker-go-sdk

## Overview

The firecracker-go-sdk provides a Go API for managing Firecracker microVMs.
It wraps the Firecracker REST API and handles VM lifecycle management.

## Key Imports

```go
import (
    firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
    "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)
```

## Core Types

### Config

```go
firecracker.Config{
    SocketPath:      "/tmp/firecracker-vm.sock",
    KernelImagePath: "/path/to/vmlinux",
    KernelArgs:      "console=ttyS0 reboot=k panic=1 pci=off init=/usr/local/bin/vulcan-guest",
    Drives: []models.Drive{
        {
            DriveID:      firecracker.String("rootfs"),
            PathOnHost:   firecracker.String("/path/to/rootfs.ext4"),
            IsRootDevice: firecracker.Bool(true),
            IsReadOnly:   firecracker.Bool(false),
        },
    },
    NetworkInterfaces: firecracker.NetworkInterfaces{
        {
            StaticConfiguration: &firecracker.StaticNetworkConfiguration{
                MacAddress:  "02:ab:cd:ef:01:23",
                HostDevName: "tap0",
            },
        },
    },
    VsockDevices: []firecracker.VsockDevice{
        {
            ID:   "vsock0",
            Path: "/tmp/vm-vsock.sock",
            CID:  3,
        },
    },
    MachineCfg: models.MachineConfiguration{
        VcpuCount:  firecracker.Int64(1),
        MemSizeMib: firecracker.Int64(128),
        Smt:        firecracker.Bool(false),
    },
    NetNS: "/var/run/netns/vulcan-vm1",
    VMID:  "vm-001",
}
```

### VsockDevice

```go
type VsockDevice struct {
    ID   string  // Device identifier
    Path string  // UDS path for host communication
    CID  uint32  // Guest context ID (>= 3)
}
```

### Helper Functions

The SDK provides pointer helpers for configuration:
- `firecracker.String(s string) *string`
- `firecracker.Bool(b bool) *bool`
- `firecracker.Int64(i int64) *int64`

## Machine Lifecycle

### Create and Start

```go
ctx := context.Background()

cfg := firecracker.Config{...}
m, err := firecracker.NewMachine(ctx, cfg,
    firecracker.WithLogger(logrus.NewEntry(logrus.New())),
)
if err != nil {
    return fmt.Errorf("create machine: %w", err)
}

if err := m.Start(ctx); err != nil {
    return fmt.Errorf("start machine: %w", err)
}
defer m.StopVMM()
```

### Shutdown vs StopVMM

- `m.Shutdown(ctx)` — graceful shutdown (sends CtrlAltDel)
- `m.StopVMM()` — force kill (sends SIGTERM to process)
- `m.Wait(ctx)` — blocks until VM exits

Pattern:
```go
// Graceful attempt, then force
if err := m.Shutdown(ctx); err != nil {
    m.StopVMM()
}
m.Wait(ctx)
```

## Boot Args

Standard Firecracker kernel arguments:
```
console=ttyS0 reboot=k panic=1 pci=off
```

For Vulcan, append the init path:
```
console=ttyS0 reboot=k panic=1 pci=off init=/usr/local/bin/vulcan-guest
```

## vsock Communication

After VM starts, Firecracker creates a UDS at the VsockDevice.Path.
Host connects to it with the CONNECT handshake (see task 4-4 vsock.go).

The UDS path is: `<socket_path>_<CID>`... actually Firecracker creates the UDS at the
configured VsockDevice.Path. The host uses `DialGuest(ctx, udsPath, port)`.

## Rootfs Handling

Original rootfs images should be read-only (shared).
Each VM needs its own copy for writes:
```go
// Copy rootfs for this VM
src := "/images/go.ext4"
dst := filepath.Join(tmpDir, "rootfs.ext4")
// cp --reflink=auto for CoW on supported filesystems
exec.Command("cp", "--reflink=auto", src, dst).Run()
```

## Network Interface

The TAP device from CNI setup is passed as HostDevName:
```go
NetworkInterfaces: firecracker.NetworkInterfaces{
    {
        StaticConfiguration: &firecracker.StaticNetworkConfiguration{
            MacAddress:  netCfg.MACAddress,
            HostDevName: netCfg.TAPDevice,
        },
    },
},
```

## Integration with Vulcan

1. `NewFirecrackerBackend(cfg Config, logger)` creates the backend
2. `Execute()` orchestrates the full VM lifecycle
3. `Cleanup()` handles resource release
4. `Capabilities()` reports supported runtimes/isolations
5. Register with `registry.Register("microvm", backend)`
