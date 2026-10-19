package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Router provides access to primary and read replica database connection pools.
type Router interface {
	Primary() *pgxpool.Pool
	Replica() *pgxpool.Pool
	Route(ctx context.Context) *pgxpool.Pool
}

type poolRouter struct {
	primary *pgxpool.Pool
	replica *pgxpool.Pool
}

var _ Router = (*poolRouter)(nil)

// NewRouter constructs a database connection pool Router.
// If replica is nil, Replica() will fall back to returning primary.
func NewRouter(primary *pgxpool.Pool, replica *pgxpool.Pool) Router {
	return &poolRouter{
		primary: primary,
		replica: replica,
	}
}

// Primary returns the connection pool dedicated to write and mutation operations.
func (r *poolRouter) Primary() *pgxpool.Pool {
	if r == nil {
		return nil
	}
	return r.primary
}

// Replica returns the connection pool for read operations.
// If replica is nil, it falls back to returning the primary pool.
func (r *poolRouter) Replica() *pgxpool.Pool {
	if r == nil {
		return nil
	}
	if r.replica != nil {
		return r.replica
	}
	return r.primary
}

// Route returns the connection pool based on context:
// if IsStickyPrimary(ctx) is true, it returns r.Primary(); otherwise r.Replica().
func (r *poolRouter) Route(ctx context.Context) *pgxpool.Pool {
	if r == nil {
		return nil
	}
	if IsStickyPrimary(ctx) {
		return r.Primary()
	}
	return r.Replica()
}

type stickyPrimaryKey struct{}

// WithStickyPrimary returns a copy of ctx that marks database reads as sticky to the primary pool.
func WithStickyPrimary(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, stickyPrimaryKey{}, true)
}

// IsStickyPrimary reports whether ctx is marked for sticky primary database routing.
func IsStickyPrimary(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	val, _ := ctx.Value(stickyPrimaryKey{}).(bool)
	return val
}
