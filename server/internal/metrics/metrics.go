// Package metrics 提供 Prometheus 指标收集和暴露功能。
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal HTTP请求总数
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devdash_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration HTTP请求耗时
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "devdash_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// CollectDuration 采集耗时
	CollectDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "devdash_collect_duration_seconds",
			Help:    "Metrics collection duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	// CronJobExecutions Cron任务执行次数
	CronJobExecutions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devdash_cronjob_executions_total",
			Help: "Total cron job executions",
		},
		[]string{"job_id", "status"},
	)

	// ActiveTerminalSessions 活跃终端会话数
	ActiveTerminalSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "devdash_terminal_sessions_active",
			Help: "Number of active terminal sessions",
		},
	)

	// ActiveAlerts 活跃告警数
	ActiveAlerts = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "devdash_alerts_active",
			Help: "Number of active alerts",
		},
	)
)
