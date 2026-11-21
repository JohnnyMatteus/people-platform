package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics agrupa as métricas Prometheus do serviço.
type Metrics struct {
	Registry        *prometheus.Registry
	HTTPRequests    *prometheus.CounterVec
	HTTPDuration    *prometheus.HistogramVec
	OutboxPublished prometheus.Counter
	OutboxFailures  prometheus.Counter
}

// NewMetrics registra as métricas num registry próprio (sem estado global).
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		Registry: reg,
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total", Help: "Requisições HTTP por rota, método e status.",
		}, []string{"route", "method", "status"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_request_duration_seconds", Help: "Latência das requisições HTTP.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
		OutboxPublished: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "outbox_events_published_total", Help: "Eventos publicados no Kafka pelo relay da outbox.",
		}),
		OutboxFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "outbox_publish_failures_total", Help: "Lotes da outbox que falharam ao publicar.",
		}),
	}
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.HTTPRequests, m.HTTPDuration, m.OutboxPublished, m.OutboxFailures)
	return m
}

// Published implementa postgres.RelayMetrics.
func (m *Metrics) Published(n int) { m.OutboxPublished.Add(float64(n)) }

// Failed implementa postgres.RelayMetrics.
func (m *Metrics) Failed() { m.OutboxFailures.Inc() }
