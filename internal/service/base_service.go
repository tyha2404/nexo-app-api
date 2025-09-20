package service

import (
	"context"

	"github.com/google/uuid"
)

// BaseService defines the common CRUD operations for all services
type BaseService[T any] interface {
	// Create a new entity
	Create(ctx context.Context, req *T) (*T, error)

	// Get retrieves an entity by its ID
	Get(ctx context.Context, id uuid.UUID) (*T, error)

	// List retrieves a paginated list of entities
	List(ctx context.Context, limit, offset int) ([]T, error)

	// Update updates an existing entity
	Update(ctx context.Context, req *T) (*T, error)

	// Delete removes an entity by its ID
	Delete(ctx context.Context, id uuid.UUID) error
}
