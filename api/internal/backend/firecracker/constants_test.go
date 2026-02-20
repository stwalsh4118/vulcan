package firecracker

import (
	"strings"
	"testing"
)

func TestRootfsPathSupported(t *testing.T) {
	tests := []struct {
		runtime string
		want    string
	}{
		{"go", "/images/go.ext4"},
		{"node", "/images/node.ext4"},
		{"python", "/images/python.ext4"},
	}
	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			got, err := RootfsPath("/images", tt.runtime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("RootfsPath(%q) = %q, want %q", tt.runtime, got, tt.want)
			}
		})
	}
}

func TestRootfsPathUnsupported(t *testing.T) {
	_, err := RootfsPath("/images", "ruby")
	if err == nil {
		t.Fatal("expected error for unsupported runtime")
	}
	if !strings.Contains(err.Error(), "unsupported runtime") {
		t.Errorf("error = %q, want it to contain 'unsupported runtime'", err.Error())
	}
}
