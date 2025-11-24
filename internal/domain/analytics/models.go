package analytics

import (
	"context"
	"time"
)

// EventType represents analytics event types
type EventType string

const (
	EventSent         EventType = "sent"
	EventDelivered    EventType = "delivered"
	EventRead         EventType = "read"
	EventClicked      EventType = "clicked"
	EventFailed       EventType = "failed"
	EventBounced      EventType = "bounced"
	EventUnsubscribed EventType = "unsubscribed"
)

// Event represents an analytics event entity
type Event struct {
	ID             string                 `json:"id" es:"event_id"`
	AppID          string                 `json:"app_id" es:"app_id"`
	NotificationID string                 `json:"notification_id" es:"notification_id"`
	UserID         string                 `json:"user_id" es:"user_id"`
	EventType      EventType              `json:"event_type" es:"event_type"`
	Channel        string                 `json:"channel" es:"channel"`
	Timestamp      time.Time              `json:"timestamp" es:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty" es:"metadata"`
}

// Filter represents query filters for analytics events
type Filter struct {
	AppID          string    `json:"app_id,omitempty"`
	NotificationID string    `json:"notification_id,omitempty"`
	UserID         string    `json:"user_id,omitempty"`
	EventType      EventType `json:"event_type,omitempty"`
	Channel        string    `json:"channel,omitempty"`
	FromDate       time.Time `json:"from_date,omitempty"`
	ToDate         time.Time `json:"to_date,omitempty"`
	Limit          int       `json:"limit,omitempty"`
	Offset         int       `json:"offset,omitempty"`
}

// Summary represents analytics summary data
type Summary struct {
	AppID          string           `json:"app_id"`
	TotalSent      int64            `json:"total_sent"`
	TotalDelivered int64            `json:"total_delivered"`
	TotalFailed    int64            `json:"total_failed"`
	DeliveryRate   float64          `json:"delivery_rate"`
	ChannelStats   map[string]int64 `json:"channel_stats"`
	Period         string           `json:"period"`
	FromDate       time.Time        `json:"from_date"`
	ToDate         time.Time        `json:"to_date"`
}

// Repository defines the interface for analytics data operations
type Repository interface {
	Track(ctx context.Context, event *Event) error
	GetEvents(ctx context.Context, filter Filter) ([]*Event, error)
	GetSummary(ctx context.Context, appID string, fromDate, toDate time.Time) (*Summary, error)
	GetChannelStats(ctx context.Context, appID string, fromDate, toDate time.Time) (map[string]int64, error)
	GetDeliveryRates(ctx context.Context, appID string, fromDate, toDate time.Time) (map[string]float64, error)
}

// Service defines the business logic interface for analytics
type Service interface {
	Track(ctx context.Context, event *Event) error
	GetEvents(ctx context.Context, filter Filter) ([]*Event, error)
	GetSummary(ctx context.Context, appID string, period string) (*Summary, error)
	GetDashboard(ctx context.Context, appID string) (*Dashboard, error)
}

// Dashboard represents analytics dashboard data
type Dashboard struct {
	Summary       *Summary               `json:"summary"`
	ChannelStats  map[string]int64       `json:"channel_stats"`
	DeliveryRates map[string]float64     `json:"delivery_rates"`
	RecentEvents  []*Event               `json:"recent_events"`
	Trends        map[string][]DataPoint `json:"trends"`
}

// DataPoint represents a time-series data point
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}
