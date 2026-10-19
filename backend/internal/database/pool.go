package database

import "github.com/jackc/pgx/v5/pgxpool"

// Router provides access to primary and read replica database connection pools.
type Router interface {
	Primary() *pgxpool.Pool
	Replica() *pgxpool.Pool
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
