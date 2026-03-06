package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// NotificationMetrics holds all notification-related Prometheus metrics
type NotificationMetrics struct {
	// Core delivery metrics
	SendTotal          *prometheus.CounterVec
	QueueDepth         *prometheus.GaugeVec
	ProcessingDuration *prometheus.HistogramVec
	RetryCount         *prometheus.CounterVec
	DeliverySuccess    *prometheus.CounterVec
	DeliveryFailure    *prometheus.CounterVec
	ProviderLatency    *prometheus.HistogramVec
	QueueLatency       *prometheus.HistogramVec

	// Phase 1: Workflow & Digest metrics
	WorkflowExecutions      *prometheus.CounterVec
	WorkflowStepDuration    *prometheus.HistogramVec
	DigestEventsAccumulated *prometheus.CounterVec
	DigestFlushes           *prometheus.CounterVec

	// Phase 2: Topic, Throttle, Auth metrics
	TopicFanout            *prometheus.CounterVec
	NotificationsThrottled *prometheus.CounterVec
	ProviderRequests       *prometheus.CounterVec
	AuthDenied             *prometheus.CounterVec
}

// NewNotificationMetrics creates and registers all notification metrics
func NewNotificationMetrics() *NotificationMetrics {
	return &NotificationMetrics{
		SendTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_send_total",
				Help: "Total number of notifications sent by channel and status",
			},
			[]string{"channel", "status", "priority"},
		),
		QueueDepth: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "notification_queue_depth",
				Help: "Current depth of notification queues by priority",
			},
			[]string{"priority"},
		),
		ProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_processing_duration_seconds",
				Help:    "Time spent processing notifications by channel",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"channel", "status"},
		),
		RetryCount: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_retry_count",
				Help: "Number of notification retry attempts by channel",
			},
			[]string{"channel", "retry_reason"},
		),
		DeliverySuccess: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_delivery_success_total",
				Help: "Total successful notification deliveries by channel",
			},
			[]string{"channel", "provider"},
		),
		DeliveryFailure: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_delivery_failure_total",
				Help: "Total failed notification deliveries by channel and reason",
			},
			[]string{"channel", "provider", "error_type"},
		),
		ProviderLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_provider_latency_seconds",
				Help:    "Latency of provider API calls by provider and channel",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"provider", "channel"},
		),
		QueueLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_queue_latency_seconds",
				Help:    "Time notifications spend in queue before processing",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"priority"},
		),

		// Phase 1: Workflow & Digest metrics
		WorkflowExecutions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "frn_workflow_executions_total",
				Help: "Total workflow executions by status (success, failure, skipped)",
			},
			[]string{"status"},
		),
		WorkflowStepDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "frn_workflow_step_duration_seconds",
				Help:    "Duration of individual workflow step execution",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"step_type"},
		),
		DigestEventsAccumulated: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "frn_digest_events_accumulated_total",
				Help: "Total events accumulated into digests",
			},
			[]string{"app_id"},
		),
		DigestFlushes: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "frn_digest_flushes_total",
				Help: "Total digest flush operations (scheduled and manual)",
			},
			[]string{"trigger"},
		),

		// Phase 2: Topic, Throttle, Auth metrics
		TopicFanout: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "frn_topic_fanout_total",
				Help: "Total notifications fanned out via topics",
			},
			[]string{"topic"},
		),
		NotificationsThrottled: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "frn_notifications_throttled_total",
				Help: "Total notifications dropped or delayed by throttle",
			},
			[]string{"app_id"},
		),
		ProviderRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "frn_provider_requests_total",
				Help: "Total outbound requests to delivery providers",
			},
			[]string{"provider", "status"},
		),
		AuthDenied: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "frn_auth_denied_total",
				Help: "Total RBAC permission denials",
			},
			[]string{"permission"},
		),
	}
}

// RecordSend increments the send total counter
func (m *NotificationMetrics) RecordSend(channel, status, priority string) {
	m.SendTotal.WithLabelValues(channel, status, priority).Inc()
}

// SetQueueDepth updates the queue depth gauge
func (m *NotificationMetrics) SetQueueDepth(priority string, depth float64) {
	m.QueueDepth.WithLabelValues(priority).Set(depth)
}

// RecordProcessingDuration records the time taken to process a notification
func (m *NotificationMetrics) RecordProcessingDuration(channel, status string, duration float64) {
	m.ProcessingDuration.WithLabelValues(channel, status).Observe(duration)
}

// RecordRetry increments the retry counter
func (m *NotificationMetrics) RecordRetry(channel, reason string) {
	m.RetryCount.WithLabelValues(channel, reason).Inc()
}

// RecordDeliverySuccess increments the delivery success counter
func (m *NotificationMetrics) RecordDeliverySuccess(channel, provider string) {
	m.DeliverySuccess.WithLabelValues(channel, provider).Inc()
}

// RecordDeliveryFailure increments the delivery failure counter
func (m *NotificationMetrics) RecordDeliveryFailure(channel, provider, errorType string) {
	m.DeliveryFailure.WithLabelValues(channel, provider, errorType).Inc()
}

// RecordProviderLatency records provider API call latency
func (m *NotificationMetrics) RecordProviderLatency(provider, channel string, duration float64) {
	m.ProviderLatency.WithLabelValues(provider, channel).Observe(duration)
}

// RecordQueueLatency records time spent in queue
func (m *NotificationMetrics) RecordQueueLatency(priority string, duration float64) {
	m.QueueLatency.WithLabelValues(priority).Observe(duration)
}

// RecordWorkflowExecution increments the workflow execution counter
func (m *NotificationMetrics) RecordWorkflowExecution(status string) {
	m.WorkflowExecutions.WithLabelValues(status).Inc()
}

// RecordWorkflowStepDuration records the duration of a workflow step
func (m *NotificationMetrics) RecordWorkflowStepDuration(stepType string, duration float64) {
	m.WorkflowStepDuration.WithLabelValues(stepType).Observe(duration)
}

// RecordDigestEvent increments the digest event accumulation counter
func (m *NotificationMetrics) RecordDigestEvent(appID string) {
	m.DigestEventsAccumulated.WithLabelValues(appID).Inc()
}

// RecordDigestFlush increments the digest flush counter
func (m *NotificationMetrics) RecordDigestFlush(trigger string) {
	m.DigestFlushes.WithLabelValues(trigger).Inc()
}

// RecordTopicFanout records notifications fanned out via a topic
func (m *NotificationMetrics) RecordTopicFanout(topic string) {
	m.TopicFanout.WithLabelValues(topic).Inc()
}

// RecordThrottle increments the throttled notification counter
func (m *NotificationMetrics) RecordThrottle(appID string) {
	m.NotificationsThrottled.WithLabelValues(appID).Inc()
}

// RecordProviderRequest records an outbound provider request
func (m *NotificationMetrics) RecordProviderRequest(provider, status string) {
	m.ProviderRequests.WithLabelValues(provider, status).Inc()
}

// RecordAuthDenied increments the RBAC permission denial counter
func (m *NotificationMetrics) RecordAuthDenied(permission string) {
	m.AuthDenied.WithLabelValues(permission).Inc()
}
