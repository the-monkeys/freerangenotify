package topic

import (
	"context"
	"time"
)

// Topic represents a named subscriber group within an application.
type Topic struct {
	ID            string    `json:"id" es:"topic_id"`
	AppID         string    `json:"app_id" es:"app_id"`
	EnvironmentID string    `json:"environment_id,omitempty" es:"environment_id"`
	Name          string    `json:"name" es:"name"`
	Key           string    `json:"key" es:"key"` // Machine-readable slug (e.g., "project-123-watchers")
	Description   string    `json:"description,omitempty" es:"description"`
	CreatedAt     time.Time `json:"created_at" es:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" es:"updated_at"`
}

// TopicSubscription links a user to a topic.
type TopicSubscription struct {
	ID            string    `json:"id" es:"subscription_id"`
	TopicID       string    `json:"topic_id" es:"topic_id"`
	AppID         string    `json:"app_id" es:"app_id"`
	EnvironmentID string    `json:"environment_id,omitempty" es:"environment_id"`
	UserID        string    `json:"user_id" es:"user_id"`
	CreatedAt     time.Time `json:"created_at" es:"created_at"`
}

// CreateRequest is the input for creating a new topic.
type CreateRequest struct {
	Name          string `json:"name" validate:"required,min=1,max=255"`
	Key           string `json:"key" validate:"required,min=1,max=128"`
	Description   string `json:"description,omitempty" validate:"max=1024"`
	EnvironmentID string `json:"environment_id,omitempty"`
}

// AddSubscribersRequest is the input for adding/removing subscribers.
type AddSubscribersRequest struct {
	UserIDs []string `json:"user_ids" validate:"required,min=1"`
}

// UpdateRequest is the input for updating a topic.
type UpdateRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1024"`
}

// Repository defines data access for topics and subscriptions.
type Repository interface {
	// Topic CRUD
	Create(ctx context.Context, topic *Topic) error
	GetByID(ctx context.Context, id string) (*Topic, error)
	GetByKey(ctx context.Context, appID, key string) (*Topic, error)
	List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*Topic, int64, error)
	Update(ctx context.Context, topic *Topic) error
	Delete(ctx context.Context, id string) error

	// Subscription management
	AddSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error
	RemoveSubscribers(ctx context.Context, topicID string, userIDs []string) error
	GetSubscribers(ctx context.Context, topicID string, limit, offset int) ([]TopicSubscription, int64, error)
	GetSubscriberCount(ctx context.Context, topicID string) (int64, error)
	GetUserTopics(ctx context.Context, appID, userID string) ([]*Topic, error)
	IsSubscribed(ctx context.Context, topicID, userID string) (bool, error)
}

// Service defines the business logic for topics.
type Service interface {
	Create(ctx context.Context, appID string, req *CreateRequest) (*Topic, error)
	Get(ctx context.Context, id, appID string) (*Topic, error)
	GetByKey(ctx context.Context, appID, key string) (*Topic, error)
	List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*Topic, int64, error)
	Update(ctx context.Context, id, appID string, req *UpdateRequest) (*Topic, error)
	Delete(ctx context.Context, id, appID string) error
	AddSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error
	RemoveSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error
	GetSubscribers(ctx context.Context, topicID, appID string, limit, offset int) ([]TopicSubscription, int64, error)
	GetSubscriberUserIDs(ctx context.Context, topicID, appID string) ([]string, error)
}
