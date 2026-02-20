# mdlayher/vsock Package Guide

**Date**: 2026-02-19
**Package**: `github.com/mdlayher/vsock`
**Version**: v1.2.1 (latest stable)
**Docs**: https://pkg.go.dev/github.com/mdlayher/vsock
**Source**: https://github.com/mdlayher/vsock
**License**: MIT

## Overview

Package `vsock` provides access to Linux VM sockets (`AF_VSOCK`) for communication
between a hypervisor and its virtual machines. The types implement the standard
`net.Listener` and `net.Conn` interfaces, making it a drop-in replacement for TCP
in VM-to-host communication scenarios.

## Constants

```go
const (
    Hypervisor = 0x0  // CID for hypervisor process
    Local      = 0x1  // CID for same-machine loopback (since Linux 5.6)
    Host       = 0x2  // CID for host processes (used by guest to dial out to host)
)
```

The `golang.org/x/sys/unix` package provides additional constants used internally:

| Constant | Value | Purpose |
|----------|-------|---------|
| `unix.VMADDR_CID_ANY` | `0xFFFFFFFF` | Wildcard CID for binding (accept from any peer) |
| `unix.VMADDR_CID_HOST` | `2` | Well-known host CID |
| `unix.VMADDR_CID_HYPERVISOR` | `0` | Well-known hypervisor CID |
| `unix.VMADDR_CID_LOCAL` | `1` | Loopback CID |
| `unix.VMADDR_PORT_ANY` | `0xFFFFFFFF` | Wildcard port for dynamic assignment |

## Types and Function Signatures

### ContextID

```go
func ContextID() (uint32, error)
```

Retrieves the local VM sockets context ID by opening `/dev/vsock` and calling
`ioctl(IOCTL_VM_SOCKETS_GET_LOCAL_CID)`. In a guest VM, this returns the CID
assigned by the hypervisor (e.g., `3` for a Firecracker guest with `"guest_cid": 3`).
Returns an error if the vsock kernel module is unavailable.

### Addr

```go
type Addr struct {
    ContextID uint32
    Port      uint32
}

func (a *Addr) Network() string  // Returns "vsock"
func (a *Addr) String() string   // Returns "vsock://<CID>:<Port>"
```

### Config

```go
type Config struct{}
```

Placeholder for future options. Pass `nil` for defaults.

### Listener (implements net.Listener)

```go
type Listener struct { /* unexported */ }

func Listen(port uint32, cfg *Config) (*Listener, error)
func ListenContextID(contextID, port uint32, cfg *Config) (*Listener, error)
func FileListener(f *os.File) (*Listener, error)

func (l *Listener) Accept() (net.Conn, error)
func (l *Listener) Addr() net.Addr
func (l *Listener) Close() error
func (l *Listener) SetDeadline(t time.Time) error
```

#### Listen

Opens a connection-oriented `net.Listener` for incoming vsock connections.
Automatically retrieves the local CID via `ContextID()` and binds to it.
If `port` is `0`, the kernel assigns a random available port (`VMADDR_PORT_ANY`).

#### ListenContextID

Advanced variant that accepts an explicit CID. Use this when you need to bind
to a specific CID or to `unix.VMADDR_CID_ANY` (0xFFFFFFFF) for accepting
connections addressed to any CID.

### Conn (implements net.Conn + syscall.Conn)

```go
type Conn struct { /* unexported */ }

func Dial(contextID, port uint32, cfg *Config) (*Conn, error)

func (c *Conn) Read(b []byte) (int, error)
func (c *Conn) Write(b []byte) (int, error)
func (c *Conn) Close() error
func (c *Conn) CloseRead() error
func (c *Conn) CloseWrite() error
func (c *Conn) LocalAddr() net.Addr
func (c *Conn) RemoteAddr() net.Addr
func (c *Conn) SetDeadline(t time.Time) error
func (c *Conn) SetReadDeadline(t time.Time) error
func (c *Conn) SetWriteDeadline(t time.Time) error
func (c *Conn) SyscallConn() (syscall.RawConn, error)
```

#### Dial

Dials a connection to a vsock listener at the specified CID and port.
From a guest, use `vsock.Host` (CID 2) to reach the host.

## net.Conn Interface Compatibility

`*vsock.Conn` fully implements `net.Conn`:

```go
var _ net.Conn = (*vsock.Conn)(nil)  // compiles
```

This means you can:
- Pass `*vsock.Conn` anywhere `net.Conn` is expected
- Wrap it in `bufio.Reader`/`bufio.Writer`
- Use `io.Copy`, `json.NewEncoder`/`json.NewDecoder`
- Set read/write deadlines for timeout control

`*vsock.Listener` fully implements `net.Listener`:

```go
var _ net.Listener = (*vsock.Listener)(nil)  // compiles
```

`Accept()` returns `net.Conn`, not `*vsock.Conn`, so existing code expecting
`net.Conn` works without type assertions.

## Guest Agent Usage Patterns

### Pattern 1: Guest Listener (Recommended for Firecracker)

The guest agent listens on a vsock port; the host connects in via Firecracker's
UDS-to-vsock bridge.

```go
package main

import (
    "log"
    "net"

    "github.com/mdlayher/vsock"
)

const vsockPort = 1024

func main() {
    // Listen() auto-detects the guest CID via /dev/vsock ioctl.
    // Internally calls ContextID() then ListenContextID(cid, port, nil).
    l, err := vsock.Listen(vsockPort, nil)
    if err != nil {
        log.Fatalf("vsock listen: %v", err)
    }
    defer l.Close()

    log.Printf("listening on vsock port %d", vsockPort)

    for {
        conn, err := l.Accept()
        if err != nil {
            log.Printf("accept error: %v", err)
            continue
        }
        go handleConnection(conn)
    }
}

func handleConnection(conn net.Conn) {
    defer conn.Close()

    buf := make([]byte, 4096)
    n, err := conn.Read(buf)
    if err != nil {
        log.Printf("read error: %v", err)
        return
    }

    // Process request...
    response := []byte("OK")
    conn.Write(response)
}
```

### Pattern 2: Guest Dials Host

If the guest needs to initiate a connection to the host:

```go
conn, err := vsock.Dial(vsock.Host, 1024, nil)
if err != nil {
    log.Fatalf("dial host: %v", err)
}
defer conn.Close()

conn.Write([]byte("hello from guest"))
```

### Pattern 3: Using with JSON Protocol (Length-Prefixed Framing)

Since `*vsock.Conn` implements `net.Conn`, it works directly with standard I/O:

```go
func handleConn(conn net.Conn) {
    defer conn.Close()

    // Read length-prefixed message
    var msgLen uint32
    if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
        return
    }

    buf := make([]byte, msgLen)
    if _, err := io.ReadFull(conn, buf); err != nil {
        return
    }

    var req GuestRequest
    if err := json.Unmarshal(buf, &req); err != nil {
        return
    }

    // Process and respond...
    resp := GuestResponse{ExitCode: 0, Output: "done"}
    respBytes, _ := json.Marshal(resp)

    binary.Write(conn, binary.BigEndian, uint32(len(respBytes)))
    conn.Write(respBytes)
}
```

### Pattern 4: Deadline-Based Timeout

```go
func handleConn(conn net.Conn, timeout time.Duration) {
    defer conn.Close()

    // Set overall deadline for the entire operation
    conn.SetDeadline(time.Now().Add(timeout))

    // Reads/writes will return os.ErrDeadlineExceeded if timeout fires
    buf := make([]byte, 4096)
    n, err := conn.Read(buf)
    if err != nil {
        // Check for timeout
        if ne, ok := err.(net.Error); ok && ne.Timeout() {
            log.Println("connection timed out")
        }
        return
    }
    _ = n
}
```

## Firecracker vsock Architecture

In Firecracker, vsock communication is bridged through a Unix Domain Socket (UDS):

```
Host Process <--AF_UNIX--> Firecracker UDS <--virtio-vsock--> Guest AF_VSOCK
```

### Host-to-Guest Connection Protocol

1. The Firecracker VM is configured with: `"vsock": { "guest_cid": 3, "uds_path": "/tmp/v.sock" }`
2. The guest agent calls `vsock.Listen(1024, nil)` and blocks on `Accept()`
3. The host connects to the UDS at `/tmp/v.sock`
4. The host sends: `CONNECT 1024\n` (ASCII text, newline-terminated)
5. Firecracker bridges the connection to the guest's vsock listener on port 1024
6. The guest's `Accept()` returns a `net.Conn`
7. Firecracker responds to the host with: `OK <host_port>\n`
8. Full-duplex byte stream is now established between host and guest

### Guest CID

Firecracker assigns the guest CID via configuration (typically `3`). The guest
discovers its own CID through `ContextID()` which reads `/dev/vsock`. The host
is always CID `2` (`vsock.Host`).

## Build Requirements

The vsock package uses `AF_VSOCK` sockets which require:
- Linux kernel with `CONFIG_VSOCKETS=y` (or `=m` with module loaded)
- `/dev/vsock` device node present
- For guests: virtio-vsock device configured by the hypervisor

Static compilation for guest binary:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o vulcan-guest ./cmd/vulcan-guest
```

The package is pure Go (no cgo), so static compilation works cleanly.

## Testing Without a VM

For unit tests, since `*vsock.Conn` implements `net.Conn`, you can substitute
any `net.Conn` in tests:

- Use `net.Pipe()` for synchronous in-memory testing
- Use Unix domain sockets for integration tests
- Abstract the listener behind an interface accepting `net.Listener`

```go
// Production: vsock listener
l, _ := vsock.Listen(1024, nil)

// Test: in-memory pipe or Unix socket
l, _ := net.Listen("unix", "/tmp/test.sock")
```

Both return `net.Listener`, so agent code that accepts `net.Listener` is
fully testable without vsock hardware.

## Sources

- https://pkg.go.dev/github.com/mdlayher/vsock
- https://github.com/mdlayher/vsock
- https://mdlayher.com/blog/linux-vm-sockets-in-go/
- https://github.com/firecracker-microvm/firecracker/blob/main/docs/vsock.md
- https://man7.org/linux/man-pages/man7/vsock.7.html
