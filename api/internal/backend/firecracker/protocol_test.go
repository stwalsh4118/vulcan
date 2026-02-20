package firecracker

import (
	"bytes"
	"testing"
)

func TestWriteReadGuestRequest(t *testing.T) {
	original := GuestRequest{
		Runtime:    "go",
		Code:       "package main\nfunc main() {}",
		Input:      []byte(`{"key":"value"}`),
		Env:        map[string]string{"GOPATH": "/go"},
		Entrypoint: "main.go",
		TimeoutS:   30,
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, &original); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	var decoded GuestRequest
	if err := ReadMessage(&buf, &decoded); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if decoded.Runtime != original.Runtime {
		t.Errorf("Runtime = %q, want %q", decoded.Runtime, original.Runtime)
	}
	if decoded.Code != original.Code {
		t.Errorf("Code = %q, want %q", decoded.Code, original.Code)
	}
	if !bytes.Equal(decoded.Input, original.Input) {
		t.Errorf("Input = %q, want %q", decoded.Input, original.Input)
	}
	if decoded.Env["GOPATH"] != "/go" {
		t.Errorf("Env[GOPATH] = %q, want /go", decoded.Env["GOPATH"])
	}
	if decoded.Entrypoint != original.Entrypoint {
		t.Errorf("Entrypoint = %q, want %q", decoded.Entrypoint, original.Entrypoint)
	}
	if decoded.TimeoutS != original.TimeoutS {
		t.Errorf("TimeoutS = %d, want %d", decoded.TimeoutS, original.TimeoutS)
	}
}

func TestWriteReadGuestResponse(t *testing.T) {
	original := GuestResponse{
		ExitCode: 0,
		Output:   "hello world\n",
		Error:    "",
		LogLines: []string{"starting...", "done"},
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, &original); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	var decoded GuestResponse
	if err := ReadMessage(&buf, &decoded); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if decoded.ExitCode != original.ExitCode {
		t.Errorf("ExitCode = %d, want %d", decoded.ExitCode, original.ExitCode)
	}
	if decoded.Output != original.Output {
		t.Errorf("Output = %q, want %q", decoded.Output, original.Output)
	}
	if decoded.Error != original.Error {
		t.Errorf("Error = %q, want %q", decoded.Error, original.Error)
	}
	if len(decoded.LogLines) != len(original.LogLines) {
		t.Fatalf("LogLines len = %d, want %d", len(decoded.LogLines), len(original.LogLines))
	}
	for i, line := range decoded.LogLines {
		if line != original.LogLines[i] {
			t.Errorf("LogLines[%d] = %q, want %q", i, line, original.LogLines[i])
		}
	}
}

func TestReadMessageTruncatedLength(t *testing.T) {
	// Only 2 bytes instead of 4 — should fail to read length prefix.
	buf := bytes.NewReader([]byte{0x00, 0x01})
	var req GuestRequest
	if err := ReadMessage(buf, &req); err == nil {
		t.Fatal("expected error for truncated length prefix")
	}
}

func TestReadMessageTruncatedPayload(t *testing.T) {
	// Length prefix says 100 bytes, but only 2 bytes of payload follow.
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x00, 0x00, 0x64}) // length = 100
	buf.Write([]byte{0x7B, 0x7D})              // "{}" — only 2 bytes

	var req GuestRequest
	if err := ReadMessage(&buf, &req); err == nil {
		t.Fatal("expected error for truncated payload")
	}
}

func TestReadMessageOversized(t *testing.T) {
	// Length prefix claims MaxMessageSize + 1 — should reject before allocating.
	var buf bytes.Buffer
	oversize := uint32(MaxMessageSize + 1)
	buf.Write([]byte{
		byte(oversize >> 24), byte(oversize >> 16),
		byte(oversize >> 8), byte(oversize),
	})

	var req GuestRequest
	if err := ReadMessage(&buf, &req); err == nil {
		t.Fatal("expected error for oversized message")
	}
}

func TestWriteReadEmptyRequest(t *testing.T) {
	original := GuestRequest{}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, &original); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	var decoded GuestRequest
	if err := ReadMessage(&buf, &decoded); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if decoded.Runtime != "" {
		t.Errorf("Runtime = %q, want empty", decoded.Runtime)
	}
	if decoded.TimeoutS != 0 {
		t.Errorf("TimeoutS = %d, want 0", decoded.TimeoutS)
	}
}
