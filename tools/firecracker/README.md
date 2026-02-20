# Firecracker microVM Build Tooling

Build scripts for the Firecracker binary, compatible kernel, and Alpine-based rootfs images.

## Prerequisites

### Host tools

| Tool | Purpose | Install (Arch) |
|------|---------|----------------|
| `curl` | Download binaries | `pacman -S curl` |
| `mkfs.ext4` | Create ext4 filesystem | `pacman -S e2fsprogs` |
| `mount` | Mount images for population | `pacman -S util-linux` |
| `dd` | Create empty disk images | (coreutils, pre-installed) |
| `tar` | Extract archives | (pre-installed) |
| `go` | Build vulcan-guest binary | `pacman -S go` |

### Privileges

- **Root/sudo** required for `mount` operations and `chroot` during rootfs builds.
- The download and guest build steps do NOT require root.

### KVM support

Firecracker requires KVM. Verify:

```bash
ls -la /dev/kvm
# crw-rw---- 1 root kvm ... /dev/kvm

# Your user must be in the kvm group:
groups | grep kvm
```

## Quick Start

```bash
# Download everything and build all images
make all

# Or step by step:
make download      # Firecracker binary + kernel
make guest         # vulcan-guest static binary
make rootfs-all    # All three rootfs images

# Verify artifacts
make verify
```

## Version Pins

| Component | Version | Variable |
|-----------|---------|----------|
| Firecracker | 1.12.0 | `FIRECRACKER_VERSION` |
| Kernel | 5.10 | `KERNEL_VERSION` |
| Alpine (major.minor) | 3.21 | `ALPINE_VERSION` |
| Alpine (patch) | 3.21.3 | `ALPINE_MINOR` |
| Rootfs size | 512 MB | `ROOTFS_SIZE_MB` |

Override at build time:

```bash
make rootfs-go ROOTFS_SIZE_MB=256
make download FIRECRACKER_VERSION=1.14.0
```

## Output Layout

```
tools/firecracker/
├── bin/
│   ├── firecracker          # Firecracker binary
│   ├── vmlinux              # Linux kernel
│   └── vulcan-guest         # Guest agent (static, amd64)
├── images/
│   ├── go.ext4              # Go runtime rootfs
│   ├── node.ext4            # Node.js runtime rootfs
│   └── python.ext4          # Python runtime rootfs
├── download.sh              # Binary downloader
├── build-rootfs.sh          # Rootfs builder
├── Makefile                 # Build orchestration
└── README.md                # This file
```

## Rootfs Contents

Each rootfs image contains:

- Alpine Linux minimal root (musl libc)
- Runtime packages (`go`, `nodejs`, or `python3`)
- `/usr/local/bin/vulcan-guest` — guest agent binary
- `/work/` — workload code extraction directory
- `/init` — init script that starts vulcan-guest as PID 1

## Clean Up

```bash
make clean    # Remove bin/ and images/
```
