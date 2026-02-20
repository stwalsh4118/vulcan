package firecracker

import "github.com/prometheus/client_golang/prometheus"

// Metric label values for workload status.
const (
	statusCompleted = "completed"
	statusFailed    = "failed"
	statusKilled    = "killed"
)

var (
	vmBootDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "vulcan_firecracker_vm_boot_seconds",
			Help:    "Duration from VM start to guest agent ready, in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	activeVMs = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "vulcan_firecracker_active_vms",
			Help: "Number of currently running Firecracker microVMs.",
		},
	)

	vsockWorkloadDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "vulcan_firecracker_vsock_workload_seconds",
			Help:    "Total vsock workload execution time from request send to final result, in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	vmCleanupDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "vulcan_firecracker_vm_cleanup_seconds",
			Help:    "Duration of VM stop and network teardown, in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	workloadsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vulcan_firecracker_workloads_total",
			Help: "Total number of workloads processed by the Firecracker backend.",
		},
		[]string{"runtime", "status"},
	)
)

func init() {
	prometheus.MustRegister(vmBootDuration)
	prometheus.MustRegister(activeVMs)
	prometheus.MustRegister(vsockWorkloadDuration)
	prometheus.MustRegister(vmCleanupDuration)
	prometheus.MustRegister(workloadsTotal)

	// Pre-initialize counter label combinations so they appear in /metrics
	// with value 0 from startup, rather than only after first observation.
	for _, rt := range SupportedRuntimes {
		workloadsTotal.WithLabelValues(rt, statusCompleted)
		workloadsTotal.WithLabelValues(rt, statusFailed)
		workloadsTotal.WithLabelValues(rt, statusKilled)
	}
}
