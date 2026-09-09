package bizbilling

import (
	"context"
	"time"
)

// Product represents a sellable item or service in the business billing module.
// Examples: "Gym Membership", "SaaS Pro Plan", "Newspaper Subscription".
type Product struct {
	ID          string                 `json:"id" es:"id"`
	AppID       string                 `json:"app_id" es:"app_id"`
	Name        string                 `json:"name" es:"name"`
	Description string                 `json:"description,omitempty" es:"description"`
	HSNCode     string                 `json:"hsn_code,omitempty" es:"hsn_code"` // HSN/SAC code for GST compliance
	TaxRate     int                    `json:"tax_rate" es:"tax_rate"`           // Tax percentage (0, 5, 12, 18, 28)
	Unit        string                 `json:"unit,omitempty" es:"unit"`         // e.g. "month", "unit", "hour"
	Active      bool                   `json:"active" es:"active"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" es:"metadata"`
	CreatedAt   time.Time              `json:"created_at" es:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" es:"updated_at"`
}

// CreateProductRequest is the request DTO for creating a product.
type CreateProductRequest struct {
	Name        string                 `json:"name" validate:"required,min=1,max=255"`
	Description string                 `json:"description,omitempty" validate:"max=1000"`
	HSNCode     string                 `json:"hsn_code,omitempty" validate:"omitempty,max=8"`
	TaxRate     int                    `json:"tax_rate" validate:"min=0,max=28"`
	Unit        string                 `json:"unit,omitempty" validate:"omitempty,max=50"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateProductRequest is the request DTO for updating a product.
type UpdateProductRequest struct {
	Name        *string                `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string                `json:"description,omitempty" validate:"omitempty,max=1000"`
	HSNCode     *string                `json:"hsn_code,omitempty" validate:"omitempty,max=8"`
	TaxRate     *int                   `json:"tax_rate,omitempty" validate:"omitempty,min=0,max=28"`
	Unit        *string                `json:"unit,omitempty" validate:"omitempty,max=50"`
	Active      *bool                  `json:"active,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProductFilter defines query filters for listing products.
type ProductFilter struct {
	AppID  string `json:"app_id"`
	Active *bool  `json:"active,omitempty"`
	Search string `json:"search,omitempty"` // Full-text search on name
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// ProductRepository defines persistence operations for products.
type ProductRepository interface {
	Create(ctx context.Context, product *Product) error
	GetByID(ctx context.Context, id string) (*Product, error)
	Update(ctx context.Context, product *Product) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ProductFilter) ([]*Product, int64, error)
}
