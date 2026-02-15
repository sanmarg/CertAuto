package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Sync metrics
	SyncTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "certauto_sync_total",
			Help: "Total number of sync operations",
		},
		[]string{"destination_type", "status"},
	)

	SyncDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "certauto_sync_duration_seconds",
			Help:    "Duration of sync operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"destination_type"},
	)

	// Certificate metrics
	CertificateExpirySeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "certauto_certificate_expiry_seconds",
			Help: "Timestamp of certificate expiry in seconds",
		},
		[]string{"namespace", "name", "destination"},
	)

	// Validation metrics
	ValidationFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "certauto_validation_failed_total",
			Help: "Total number of certificate validation failures",
		},
		[]string{"namespace", "name", "reason"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		SyncTotal,
		SyncDuration,
		CertificateExpirySeconds,
		ValidationFailedTotal,
	)
}
