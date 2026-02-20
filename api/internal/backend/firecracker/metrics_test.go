package firecracker

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsRegistered(t *testing.T) {
	// Verify all metrics are registered with the default registerer.
	// If any were not registered, Gather would not include them.
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	expected := []string{
		"vulcan_firecracker_vm_boot_seconds",
		"vulcan_firecracker_active_vms",
		"vulcan_firecracker_vsock_workload_seconds",
		"vulcan_firecracker_vm_cleanup_seconds",
		"vulcan_firecracker_workloads_total",
	}

	found := make(map[string]bool)
	for _, fam := range families {
		found[fam.GetName()] = true
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("metric %q not registered", name)
		}
	}
}

func TestWorkloadsTotalLabels(t *testing.T) {
	// Record a completed and a failed workload.
	workloadsTotal.WithLabelValues("go", statusCompleted).Inc()
	workloadsTotal.WithLabelValues("go", statusFailed).Inc()
	workloadsTotal.WithLabelValues("node", statusCompleted).Inc()

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var wlFamily *dto.MetricFamily
	for _, fam := range families {
		if fam.GetName() == "vulcan_firecracker_workloads_total" {
			wlFamily = fam
			break
		}
	}
	if wlFamily == nil {
		t.Fatal("workloads_total metric family not found")
	}

	// Verify we have at least the labels we just recorded.
	if len(wlFamily.GetMetric()) < 3 {
		t.Errorf("expected at least 3 metric series, got %d", len(wlFamily.GetMetric()))
	}
}

func TestActiveVMsGauge(t *testing.T) {
	// Reset gauge to known state.
	activeVMs.Set(0)

	activeVMs.Inc()
	activeVMs.Inc()
	activeVMs.Dec()

	val := getGaugeValue(t, "vulcan_firecracker_active_vms")
	if val != 1 {
		t.Errorf("activeVMs gauge = %f, want 1", val)
	}

	// Reset.
	activeVMs.Set(0)
}

func TestBootDurationObserved(t *testing.T) {
	vmBootDuration.Observe(0.125)

	count := getHistogramCount(t, "vulcan_firecracker_vm_boot_seconds")
	if count == 0 {
		t.Error("vmBootDuration has no observations")
	}
}

func TestCleanupDurationObserved(t *testing.T) {
	vmCleanupDuration.Observe(0.050)

	count := getHistogramCount(t, "vulcan_firecracker_vm_cleanup_seconds")
	if count == 0 {
		t.Error("vmCleanupDuration has no observations")
	}
}

func TestVsockWorkloadDurationObserved(t *testing.T) {
	vsockWorkloadDuration.Observe(0.010)

	count := getHistogramCount(t, "vulcan_firecracker_vsock_workload_seconds")
	if count == 0 {
		t.Error("vsockWorkloadDuration has no observations")
	}
}

func getGaugeValue(t *testing.T, name string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() == name {
			metrics := fam.GetMetric()
			if len(metrics) > 0 && metrics[0].GetGauge() != nil {
				return metrics[0].GetGauge().GetValue()
			}
		}
	}
	t.Fatalf("gauge %q not found", name)
	return 0
}

func getHistogramCount(t *testing.T, name string) uint64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() == name {
			metrics := fam.GetMetric()
			if len(metrics) > 0 && metrics[0].GetHistogram() != nil {
				return metrics[0].GetHistogram().GetSampleCount()
			}
		}
	}
	t.Fatalf("histogram %q not found", name)
	return 0
}
