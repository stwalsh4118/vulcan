package model

import (
	"regexp"
	"testing"
)

// crockfordBase32 matches valid ULID strings (26 chars, Crockford Base32 alphabet).
var crockfordBase32 = regexp.MustCompile(`^[0123456789ABCDEFGHJKMNPQRSTVWXYZ]{26}$`)

func TestNewIDFormat(t *testing.T) {
	id := NewID()
	if !crockfordBase32.MatchString(id) {
		t.Errorf("NewID() = %q, does not match Crockford Base32 ULID format", id)
	}
}

func TestNewIDUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewID()
		if seen[id] {
			t.Fatalf("NewID() produced duplicate: %s", id)
		}
		seen[id] = true
	}
}

func TestStatusConstants(t *testing.T) {
	statuses := []struct {
		constant string
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusKilled, "killed"},
	}
	for _, s := range statuses {
		if s.constant != s.expected {
			t.Errorf("status constant = %q, want %q", s.constant, s.expected)
		}
	}
}

func TestIsolationConstants(t *testing.T) {
	isolations := []struct {
		constant string
		expected string
	}{
		{IsolationMicroVM, "microvm"},
		{IsolationIsolate, "isolate"},
		{IsolationGVisor, "gvisor"},
		{IsolationAuto, "auto"},
	}
	for _, iso := range isolations {
		if iso.constant != iso.expected {
			t.Errorf("isolation constant = %q, want %q", iso.constant, iso.expected)
		}
	}
}

func TestValidTransition(t *testing.T) {
	valid := []struct {
		from, to string
	}{
		{StatusPending, StatusRunning},
		{StatusPending, StatusFailed},
		{StatusPending, StatusKilled},
		{StatusRunning, StatusCompleted},
		{StatusRunning, StatusFailed},
		{StatusRunning, StatusKilled},
	}
	for _, tc := range valid {
		if !ValidTransition(tc.from, tc.to) {
			t.Errorf("ValidTransition(%q, %q) = false, want true", tc.from, tc.to)
		}
	}

	invalid := []struct {
		from, to string
	}{
		{StatusPending, StatusCompleted},
		{StatusCompleted, StatusRunning},
		{StatusCompleted, StatusPending},
		{StatusFailed, StatusPending},
		{StatusFailed, StatusRunning},
		{StatusKilled, StatusRunning},
		{StatusKilled, StatusPending},
		{StatusRunning, StatusPending},
	}
	for _, tc := range invalid {
		if ValidTransition(tc.from, tc.to) {
			t.Errorf("ValidTransition(%q, %q) = true, want false", tc.from, tc.to)
		}
	}
}

func TestRuntimeConstants(t *testing.T) {
	runtimes := []struct {
		constant string
		expected string
	}{
		{RuntimeGo, "go"},
		{RuntimeNode, "node"},
		{RuntimePython, "python"},
		{RuntimeWasm, "wasm"},
		{RuntimeOCI, "oci"},
	}
	for _, rt := range runtimes {
		if rt.constant != rt.expected {
			t.Errorf("runtime constant = %q, want %q", rt.constant, rt.expected)
		}
	}
}
