# CNI Library Guide for Task 4-5

**Date**: 2026-02-19
**Package**: `github.com/containernetworking/cni`
**Version**: v1.2.3 (latest stable)
**Docs**: https://pkg.go.dev/github.com/containernetworking/cni

## Overview

The `containernetworking/cni` package provides `libcni`, a Go library for programmatically
invoking CNI plugins. It handles loading conflist files, executing plugin chains (ADD/DEL/CHECK),
and parsing results.

## Key Imports

```go
import (
    "github.com/containernetworking/cni/libcni"
    "github.com/containernetworking/cni/pkg/types"
    types100 "github.com/containernetworking/cni/pkg/types/100"
)
```

## Core API

### CNIConfig

The main entry point for invoking CNI plugins:

```go
cniConfig := libcni.NewCNIConfigWithCacheDir(
    []string{"/opt/cni/bin"},  // plugin binary directories
    "/var/lib/cni/cache",       // cache directory for results
    nil,                         // exec interface (nil = default)
)
```

### ConfListFromBytes

Parse a CNI conflist JSON into a NetworkConfigList:

```go
confBytes := []byte(`{
    "cniVersion": "1.0.0",
    "name": "vulcan-net",
    "plugins": [...]
}`)
confList, err := libcni.ConfListFromBytes(confBytes)
```

### RuntimeConf

Provides per-invocation context:

```go
rtConf := &libcni.RuntimeConf{
    ContainerID: "vulcan-vm-abc123",           // unique ID
    NetNS:       "/var/run/netns/vulcan-abc",  // network namespace path
    IfName:      "eth0",                        // interface name inside NS
}
```

### AddNetworkList / DelNetworkList

Execute the plugin chain:

```go
// ADD: create network interfaces
result, err := cniConfig.AddNetworkList(ctx, confList, rtConf)

// DEL: tear down network interfaces
err := cniConfig.DelNetworkList(ctx, confList, rtConf)
```

## Parsing ADD Results

The ADD result is an opaque `types.Result`. Convert to concrete type:

```go
result, err := cniConfig.AddNetworkList(ctx, confList, rtConf)
if err != nil {
    return err
}

// Convert to typed result
res, err := types100.NewResultFromResult(result)
if err != nil {
    return err
}

// Access network details
for _, iface := range res.Interfaces {
    fmt.Printf("Interface: %s (MAC: %s)\n", iface.Name, iface.Mac)
}
for _, ip := range res.IPs {
    fmt.Printf("IP: %s Gateway: %s\n", ip.Address.String(), ip.Gateway)
}
```

### types100.Result Structure

```go
type Result struct {
    CNIVersion string
    Interfaces []*Interface  // name, mac, sandbox (netns path)
    IPs        []*IPConfig   // address (net.IPNet), gateway (net.IP), interface index
    Routes     []*types.Route // dst (net.IPNet), gw (net.IP)
    DNS        types.DNS      // nameservers, domain, search, options
}
```

## Conflist Format: Bridge + tc-redirect-tap

For Firecracker microVMs, the standard plugin chain is:

1. **bridge**: Creates a veth pair, one end in the namespace, one on a host bridge
2. **tc-redirect-tap**: Converts the veth inside the namespace into a TAP device

```json
{
    "cniVersion": "1.0.0",
    "name": "vulcan-fcnet",
    "plugins": [
        {
            "type": "bridge",
            "bridge": "fcbr0",
            "isGateway": true,
            "ipMasq": true,
            "ipam": {
                "type": "host-local",
                "subnet": "10.168.0.0/24",
                "gateway": "10.168.0.1"
            }
        },
        {
            "type": "tc-redirect-tap"
        }
    ]
}
```

**Bridge plugin fields**:
- `bridge`: Host bridge device name
- `isGateway`: Assign gateway IP to bridge interface
- `ipMasq`: Set up IP masquerade for traffic from this network
- `ipam.type`: IP address management plugin (`host-local` for local allocation)
- `ipam.subnet`: CIDR subnet for IP allocation
- `ipam.gateway`: Gateway IP within the subnet

**tc-redirect-tap**: No configuration needed — it takes the veth from the bridge plugin
and converts it to a TAP device that Firecracker can use for VM networking.

## Network Namespace Management

CNI expects the network namespace to already exist. Create with:

```go
import "golang.org/x/sys/unix"

// Create namespace directory
os.MkdirAll("/var/run/netns", 0o755)

// Create namespace file
nsPath := "/var/run/netns/vulcan-" + vmID
f, _ := os.Create(nsPath)
f.Close()

// Bind-mount a new network namespace
// (typically done via ip netns or runtime.LockOSThread + unshare)
```

In practice, use `ip netns add <name>` via exec or the netns package for namespace creation.

## Error Handling Patterns

- `AddNetworkList` may return partial results on error — always attempt cleanup
- `DelNetworkList` is idempotent — safe to call multiple times
- Check for `*types.Error` for structured CNI error codes

## Vulcan Integration Points

- **Config.CNIConfigDir**: Where conflist JSON is written
- **Config.CNIBinDir**: Where CNI plugin binaries live (bridge, tc-redirect-tap, host-local)
- **NetworkManager.Setup()**: Creates netns → writes conflist → calls AddNetworkList → parses result
- **NetworkManager.Teardown()**: Calls DelNetworkList → removes netns
