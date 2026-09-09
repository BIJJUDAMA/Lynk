package review

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReviewRepository defines the persistence operations for reviews and aggregate rating summaries.
type ReviewRepository interface {
	CreateReview(ctx context.Context, review *Review) error
	GetReviewsByContractID(ctx context.Context, contractID uuid.UUID) ([]*Review, error)
	GetUserReviewsWithSummary(ctx context.Context, userID string) (*UserReviewSummary, error)
	HasUserReviewedContract(ctx context.Context, contractID uuid.UUID, reviewerID string) (bool, error)
}

// Repository implements ReviewRepository backed by PostgreSQL with pgxpool.Pool.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new review repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ ReviewRepository = (*Repository)(nil)

// CreateReview inserts a new review record into PostgreSQL.
// Enforces duplicate review prevention via unique constraint uq_contract_reviewer.
func (r *Repository) CreateReview(ctx context.Context, review *Review) error {
	if review.ID == uuid.Nil {
		review.ID = uuid.New()
	}

	query := `
		INSERT INTO reviews (
			id, contract_id, reviewer_id, reviewee_id, rating, comment, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING created_at;
	`
	err := r.db.QueryRow(ctx, query,
		review.ID,
		review.ContractID,
		review.ReviewerID,
		review.RevieweeID,
		review.Rating,
		review.Comment,
	).Scan(&review.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateReview
		}
		return fmt.Errorf("insert review: %w", err)
	}

	return nil
}

const queryGetReviewsByContractID = `
		SELECT 
			r.id, r.contract_id, r.reviewer_id, r.reviewee_id, r.rating, r.comment, r.created_at,
			u1.id, COALESCE(p1.first_name, ''), COALESCE(p1.last_name, ''), u1.role,
			u2.id, COALESCE(p2.first_name, ''), COALESCE(p2.last_name, ''), u2.role
		FROM reviews r
		JOIN users u1 ON r.reviewer_id = u1.id
		LEFT JOIN profiles p1 ON p1.user_id = u1.id
		JOIN users u2 ON r.reviewee_id = u2.id
		LEFT JOIN profiles p2 ON p2.user_id = u2.id
		WHERE r.contract_id = $1
		ORDER BY r.created_at ASC;
	`

const queryGetUserReviews = `
		SELECT 
			r.id, r.contract_id, r.reviewer_id, r.reviewee_id, r.rating, r.comment, r.created_at,
			u.id, COALESCE(p.first_name, ''), COALESCE(p.last_name, ''), u.role
		FROM reviews r
		JOIN users u ON r.reviewer_id = u.id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE r.reviewee_id = $1
		ORDER BY r.created_at DESC;
	`

func getContractReviewsQuery() string {
	return queryGetReviewsByContractID
}

func getUserReviewsQuery() string {
	return queryGetUserReviews
}

// GetReviewsByContractID retrieves all reviews submitted for a specific contract.
func (r *Repository) GetReviewsByContractID(ctx context.Context, contractID uuid.UUID) ([]*Review, error) {
	query := queryGetReviewsByContractID

	rows, err := r.db.Query(ctx, query, contractID)
	if err != nil {
		return nil, fmt.Errorf("query contract reviews: %w", err)
	}
	defer rows.Close()

	reviews := make([]*Review, 0)
	for rows.Next() {
		var rev Review
		var reviewer UserSummary
		var reviewee UserSummary

		err := rows.Scan(
			&rev.ID, &rev.ContractID, &rev.ReviewerID, &rev.RevieweeID, &rev.Rating, &rev.Comment, &rev.CreatedAt,
			&reviewer.ID, &reviewer.FirstName, &reviewer.LastName, &reviewer.Role,
			&reviewee.ID, &reviewee.FirstName, &reviewee.LastName, &reviewee.Role,
		)
		if err != nil {
			return nil, fmt.Errorf("scan review row: %w", err)
		}

		rev.Reviewer = &reviewer
		rev.Reviewee = &reviewee
		reviews = append(reviews, &rev)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return reviews, nil
}

// GetUserReviewsWithSummary calculates average rating, review count, and retrieves all reviews received by a user.
func (r *Repository) GetUserReviewsWithSummary(ctx context.Context, userID string) (*UserReviewSummary, error) {
	// First verify that the user exists in users table
	var userExists bool
	userCheckQuery := `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1);`
	if err := r.db.QueryRow(ctx, userCheckQuery, userID).Scan(&userExists); err != nil {
		return nil, fmt.Errorf("check user existence: %w", err)
	}
	if !userExists {
		return nil, ErrUserNotFound
	}

	// Query aggregated rating and count
	var avgRating float64
	var reviewCount int
	aggQuery := `
		SELECT 
			COALESCE(AVG(rating)::numeric, 0.0),
			COUNT(*)
		FROM reviews
		WHERE reviewee_id = $1;
	`
	if err := r.db.QueryRow(ctx, aggQuery, userID).Scan(&avgRating, &reviewCount); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("query review aggregation: %w", err)
		}
	}

	// Round average rating to 2 decimal places
	avgRating = math.Round(avgRating*100) / 100

	// Query the detailed reviews list
	reviewsQuery := queryGetUserReviews

	rows, err := r.db.Query(ctx, reviewsQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query user reviews: %w", err)
	}
	defer rows.Close()

	reviews := make([]*Review, 0)
	for rows.Next() {
		var rev Review
		var reviewer UserSummary

		err := rows.Scan(
			&rev.ID, &rev.ContractID, &rev.ReviewerID, &rev.RevieweeID, &rev.Rating, &rev.Comment, &rev.CreatedAt,
			&reviewer.ID, &reviewer.FirstName, &reviewer.LastName, &reviewer.Role,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user review row: %w", err)
		}

		rev.Reviewer = &reviewer
		reviews = append(reviews, &rev)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return &UserReviewSummary{
		UserID:        userID,
		AverageRating: avgRating,
		ReviewCount:   reviewCount,
		Reviews:       reviews,
	}, nil
}

// HasUserReviewedContract checks if a reviewer has already submitted a review for a contract.
func (r *Repository) HasUserReviewedContract(ctx context.Context, contractID uuid.UUID, reviewerID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM reviews WHERE contract_id = $1 AND reviewer_id = $2);`
	err := r.db.QueryRow(ctx, query, contractID, reviewerID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check existing review: %w", err)
	}
	return exists, nil
}
