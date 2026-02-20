package firecracker

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// MaxMessageSize is the maximum allowed vsock message payload (16 MiB).
const MaxMessageSize = 16 << 20

// GuestRequest is the JSON payload sent from host to guest over vsock.
type GuestRequest struct {
	Runtime     string            `json:"runtime"`
	Code        string            `json:"code"`
	CodeArchive []byte            `json:"code_archive,omitempty"`
	Input       []byte            `json:"input,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Entrypoint  string            `json:"entrypoint,omitempty"`
	TimeoutS    int               `json:"timeout_s"`
}

// GuestResponse is the JSON payload sent from guest to host over vsock.
type GuestResponse struct {
	ExitCode int      `json:"exit_code"`
	Output   string   `json:"output"`
	Error    string   `json:"error,omitempty"`
	LogLines []string `json:"log_lines,omitempty"`
}

// Guest→host message types for vsock streaming.
const (
	MsgTypeLog    = "log"
	MsgTypeResult = "result"
)

// GuestMessage is the envelope for all guest→host messages over vsock.
// During execution, the guest sends log lines with Type="log".
// After execution completes, the guest sends one final message with Type="result".
type GuestMessage struct {
	Type     string         `json:"type"`
	Line     string         `json:"line,omitempty"`
	Response *GuestResponse `json:"response,omitempty"`
}

// WriteMessage writes a length-prefixed JSON message to w.
// The frame format is: 4-byte big-endian length prefix followed by the JSON payload.
func WriteMessage(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	length := uint32(len(data))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("write length prefix: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

// ReadMessage reads a length-prefixed JSON message from r and decodes it into v.
func ReadMessage(r io.Reader, v any) error {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return fmt.Errorf("read length prefix: %w", err)
	}

	if length > MaxMessageSize {
		return fmt.Errorf("message size %d exceeds maximum %d", length, MaxMessageSize)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal message: %w", err)
	}

	return nil
}
