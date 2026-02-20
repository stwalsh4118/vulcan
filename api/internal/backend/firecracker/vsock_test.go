package firecracker

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGuestConnSendAndReceive(t *testing.T) {
	server, client := net.Pipe()
	gc := &GuestConn{conn: client, reader: client}

	req := GuestRequest{
		Runtime:  "node",
		Code:     `console.log("test")`,
		TimeoutS: 10,
	}

	expectedResp := GuestResponse{
		ExitCode: 0,
		Output:   "test\n",
	}

	// Mock guest: read request, send result.
	go func() {
		var gotReq GuestRequest
		if err := ReadMessage(server, &gotReq); err != nil {
			t.Errorf("mock read: %v", err)
			return
		}
		if gotReq.Runtime != req.Runtime {
			t.Errorf("Runtime = %q, want %q", gotReq.Runtime, req.Runtime)
		}

		msg := GuestMessage{Type: MsgTypeResult, Response: &expectedResp}
		if err := WriteMessage(server, &msg); err != nil {
			t.Errorf("mock write: %v", err)
		}
		server.Close()
	}()

	resp, err := gc.RunWorkload(req, nil)
	if err != nil {
		t.Fatalf("RunWorkload: %v", err)
	}
	if resp.ExitCode != expectedResp.ExitCode {
		t.Errorf("ExitCode = %d, want %d", resp.ExitCode, expectedResp.ExitCode)
	}
	if resp.Output != expectedResp.Output {
		t.Errorf("Output = %q, want %q", resp.Output, expectedResp.Output)
	}
}

func TestGuestConnLogStreaming(t *testing.T) {
	server, client := net.Pipe()
	gc := &GuestConn{conn: client, reader: client}

	logLines := []string{"starting...", "processing...", "done!"}

	// Mock guest: send log lines then result.
	go func() {
		var gotReq GuestRequest
		ReadMessage(server, &gotReq)

		for _, line := range logLines {
			msg := GuestMessage{Type: MsgTypeLog, Line: line}
			WriteMessage(server, &msg)
		}

		msg := GuestMessage{
			Type:     MsgTypeResult,
			Response: &GuestResponse{ExitCode: 0, Output: "ok"},
		}
		WriteMessage(server, &msg)
		server.Close()
	}()

	var mu sync.Mutex
	var receivedLogs []string
	logWriter := func(line string) {
		mu.Lock()
		receivedLogs = append(receivedLogs, line)
		mu.Unlock()
	}

	resp, err := gc.RunWorkload(GuestRequest{Runtime: "node"}, logWriter)
	if err != nil {
		t.Fatalf("RunWorkload: %v", err)
	}

	if resp.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", resp.ExitCode)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(receivedLogs) != len(logLines) {
		t.Fatalf("received %d log lines, want %d", len(receivedLogs), len(logLines))
	}
	for i, line := range receivedLogs {
		if line != logLines[i] {
			t.Errorf("log[%d] = %q, want %q", i, line, logLines[i])
		}
	}
}

func TestGuestConnConnectionReset(t *testing.T) {
	server, client := net.Pipe()
	gc := &GuestConn{conn: client, reader: client}

	// Mock guest: close connection after reading request (simulates crash).
	go func() {
		var gotReq GuestRequest
		ReadMessage(server, &gotReq)
		server.Close()
	}()

	_, err := gc.RunWorkload(GuestRequest{Runtime: "node"}, nil)
	if err == nil {
		t.Fatal("expected error for connection reset")
	}
}

func TestGuestConnNilResult(t *testing.T) {
	server, client := net.Pipe()
	gc := &GuestConn{conn: client, reader: client}

	// Mock guest: send result with nil Response.
	go func() {
		var gotReq GuestRequest
		ReadMessage(server, &gotReq)
		msg := GuestMessage{Type: MsgTypeResult, Response: nil}
		WriteMessage(server, &msg)
		server.Close()
	}()

	_, err := gc.RunWorkload(GuestRequest{Runtime: "node"}, nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
	if !strings.Contains(err.Error(), "nil response") {
		t.Errorf("error = %q, want to contain 'nil response'", err.Error())
	}
}

func TestGuestConnUnknownMessageType(t *testing.T) {
	server, client := net.Pipe()
	gc := &GuestConn{conn: client, reader: client}

	// Mock guest: send unknown message type.
	go func() {
		var gotReq GuestRequest
		ReadMessage(server, &gotReq)
		msg := GuestMessage{Type: "unknown_type"}
		WriteMessage(server, &msg)
		server.Close()
	}()

	_, err := gc.RunWorkload(GuestRequest{Runtime: "node"}, nil)
	if err == nil {
		t.Fatal("expected error for unknown message type")
	}
	if !strings.Contains(err.Error(), "unknown message type") {
		t.Errorf("error = %q, want to contain 'unknown message type'", err.Error())
	}
}

func TestDialGuestContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := DialGuest(ctx, "/nonexistent.sock", 1024)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDialGuestRetries(t *testing.T) {
	// Create a Unix listener that will reject connections initially,
	// then accept on the 3rd attempt.
	sockPath := t.TempDir() + "/test.sock"

	// Start listening after a short delay to simulate "guest not ready".
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(250 * time.Millisecond) // Wait for 2 retry attempts
		l, err := net.Listen("unix", sockPath)
		if err != nil {
			t.Errorf("listen: %v", err)
			return
		}
		defer l.Close()

		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Handle the CONNECT handshake.
		buf := make([]byte, 256)
		n, _ := conn.Read(buf)
		_ = n
		conn.Write([]byte("OK 1024\n"))

		// Keep connection open for the test to complete.
		time.Sleep(time.Second)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gc, err := DialGuest(ctx, sockPath, 1024)
	if err != nil {
		t.Fatalf("DialGuest: %v", err)
	}
	gc.Close()
	wg.Wait()
}
