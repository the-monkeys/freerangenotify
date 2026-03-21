package resourcelink

import (
	"context"
	"time"
)

// ResourceType identifies a linkable resource kind.
type ResourceType string

const (
	TypeUser     ResourceType = "users"
	TypeTemplate ResourceType = "templates"
	TypeWorkflow ResourceType = "workflows"
	TypeDigest   ResourceType = "digest_rules"
	TypeTopic    ResourceType = "topics"
	TypeProvider ResourceType = "providers"
)

// ValidTypes enumerates every resource type that can be linked.
var ValidTypes = map[ResourceType]bool{
	TypeUser: true, TypeTemplate: true, TypeWorkflow: true,
	TypeDigest: true, TypeTopic: true, TypeProvider: true,
}

// Link represents a cross-app resource reference.  The resource itself lives
// in SourceAppID; TargetAppID sees it as a read-only linked resource.
type Link struct {
	LinkID       string       `json:"link_id" es:"link_id"`
	TargetAppID  string       `json:"target_app_id" es:"target_app_id"`
	SourceAppID  string       `json:"source_app_id" es:"source_app_id"`
	ResourceType ResourceType `json:"resource_type" es:"resource_type"`
	ResourceID   string       `json:"resource_id" es:"resource_id"`
	LinkedBy     string       `json:"linked_by" es:"linked_by"`
	LinkedAt     time.Time    `json:"linked_at" es:"linked_at"`
}

// ImportRequest is the payload for POST /v1/apps/:target_id/import.
type ImportRequest struct {
	SourceAppID   string         `json:"source_app_id" validate:"required"`
	ResourceTypes []ResourceType `json:"resources" validate:"required,min=1"`
}

// ImportResult summarizes a completed import operation.
type ImportResult struct {
	Linked  map[ResourceType]int `json:"linked"`
	Skipped map[ResourceType]int `json:"skipped"`
}

// Repository persists and queries resource links.
type Repository interface {
	Create(ctx context.Context, link *Link) error
	Delete(ctx context.Context, linkID string) error
	DeleteByTargetAndResource(ctx context.Context, targetAppID string, resourceType ResourceType, resourceID string) error
	DeleteAllByTarget(ctx context.Context, targetAppID string) error

	// GetLinkedAppIDs returns the source app IDs that have resources of the
	// given type linked into targetAppID.
	GetLinkedAppIDs(ctx context.Context, targetAppID string, resourceType ResourceType) ([]string, error)

	// GetLinkedResourceIDs returns resource IDs of the given type linked
	// from sourceAppID into targetAppID.
	GetLinkedResourceIDs(ctx context.Context, targetAppID, sourceAppID string, resourceType ResourceType) ([]string, error)

	// Exists checks whether a specific resource is already linked.
	Exists(ctx context.Context, targetAppID string, resourceType ResourceType, resourceID string) (bool, error)

	// ListByTarget returns all links for a target app, optionally filtered by type.
	ListByTarget(ctx context.Context, targetAppID string, resourceType *ResourceType) ([]*Link, error)

	// ListBySource returns all links where this app is the source (its resources are shared out).
	ListBySource(ctx context.Context, sourceAppID string, resourceType *ResourceType) ([]*Link, error)

	// DeleteAllBySource deletes all links where this app is the source.
	DeleteAllBySource(ctx context.Context, sourceAppID string) error

	// CountByResource counts how many target apps link to a specific resource from a specific source.
	CountByResource(ctx context.Context, sourceAppID string, resourceType ResourceType, resourceID string) (int64, error)

	// UpdateLink updates a link document in place.
	UpdateLink(ctx context.Context, link *Link) error

	// BulkCreate creates multiple links in one request.
	BulkCreate(ctx context.Context, links []*Link) error
}
