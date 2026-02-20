package firecracker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// Retry defaults for vsock connection establishment.
const (
	dialMaxRetries  = 5
	dialBaseBackoff = 100 * time.Millisecond
)

// GuestConn wraps a connection to the guest agent inside a Firecracker microVM.
// Each GuestConn is used by a single goroutine.
type GuestConn struct {
	conn   net.Conn
	reader io.Reader // buffered reader preserving any bytes read ahead during handshake
}

// DialGuest connects to the guest agent via Firecracker's vsock UDS bridge.
// The udsPath is the Unix socket created by Firecracker for vsock communication.
// The port is the vsock port the guest agent listens on.
// Retries with exponential backoff on connection failure.
func DialGuest(ctx context.Context, udsPath string, port uint32) (*GuestConn, error) {
	var lastErr error
	backoff := dialBaseBackoff

	for attempt := range dialMaxRetries {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("dial guest: %w", ctx.Err())
		default:
		}

		gc, err := dialVsockUDS(ctx, udsPath, port)
		if err != nil {
			lastErr = err
			if attempt < dialMaxRetries-1 {
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return nil, fmt.Errorf("dial guest: %w", ctx.Err())
				}
				backoff *= 2
			}
			continue
		}

		// Set overall deadline from context if present.
		if deadline, ok := ctx.Deadline(); ok {
			if err := gc.conn.SetDeadline(deadline); err != nil {
				gc.conn.Close()
				return nil, fmt.Errorf("set deadline: %w", err)
			}
		}

		return gc, nil
	}

	return nil, fmt.Errorf("dial guest after %d attempts: %w", dialMaxRetries, lastErr)
}

// dialVsockUDS connects to Firecracker's UDS and sends the CONNECT handshake.
// Firecracker bridges the UDS connection to the guest's vsock listener.
// Protocol: send "CONNECT <port>\n", receive "OK <host_port>\n".
// Returns a GuestConn with a buffered reader to prevent protocol desynchronization.
func dialVsockUDS(ctx context.Context, udsPath string, port uint32) (*GuestConn, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", udsPath)
	if err != nil {
		return nil, fmt.Errorf("connect to UDS %s: %w", udsPath, err)
	}

	// Send CONNECT handshake.
	connectMsg := fmt.Sprintf("CONNECT %d\n", port)
	if _, err := conn.Write([]byte(connectMsg)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send CONNECT: %w", err)
	}

	// Read response (expect "OK <port>\n").
	// Use a buffered reader and keep it for all subsequent reads to avoid
	// losing bytes that the buffer may have read ahead.
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read CONNECT response: %w", err)
	}

	response = strings.TrimSpace(response)
	if !strings.HasPrefix(response, "OK ") {
		conn.Close()
		return nil, fmt.Errorf("vsock CONNECT failed: %s", response)
	}

	return &GuestConn{conn: conn, reader: reader}, nil
}

// SendWorkload sends a GuestRequest to the guest agent using length-prefixed JSON framing.
func (gc *GuestConn) SendWorkload(req GuestRequest) error {
	if err := WriteMessage(gc.conn, &req); err != nil {
		return fmt.Errorf("send workload: %w", err)
	}
	return nil
}

// RunWorkload sends a workload request and reads back streaming log lines and the final result.
// Each log line is passed to logWriter in real time. Returns the final GuestResponse.
func (gc *GuestConn) RunWorkload(req GuestRequest, logWriter func(string)) (GuestResponse, error) {
	if err := gc.SendWorkload(req); err != nil {
		return GuestResponse{}, err
	}
	return gc.readMessages(logWriter)
}

// readMessages reads GuestMessage frames from the connection in a loop.
// Log lines are delivered to logWriter; the final result message terminates the loop.
func (gc *GuestConn) readMessages(logWriter func(string)) (GuestResponse, error) {
	for {
		var msg GuestMessage
		if err := ReadMessage(gc.reader, &msg); err != nil {
			return GuestResponse{}, fmt.Errorf("read guest message: %w", err)
		}

		switch msg.Type {
		case MsgTypeLog:
			if logWriter != nil {
				logWriter(msg.Line)
			}
		case MsgTypeResult:
			if msg.Response == nil {
				return GuestResponse{}, fmt.Errorf("received result message with nil response")
			}
			return *msg.Response, nil
		default:
			return GuestResponse{}, fmt.Errorf("unknown message type: %q", msg.Type)
		}
	}
}

// Close closes the underlying connection.
func (gc *GuestConn) Close() error {
	return gc.conn.Close()
}
