package team

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access operations for project membership and invitations.
type Repository interface {
	AddMember(ctx context.Context, pm *ProjectMember) error
	GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*ProjectMember, error)
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error)
	UpdateRole(ctx context.Context, projectID uuid.UUID, memberID string, role Role) error
	RemoveMember(ctx context.Context, projectID uuid.UUID, memberID string) error
	CreateInvitation(ctx context.Context, inv *ProjectInvitation) error
	GetInvitation(ctx context.Context, id uuid.UUID) (*ProjectInvitation, error)
	UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status string) error
	CountLeads(ctx context.Context, projectID uuid.UUID) (int, error)
	TransferLeadershipTx(ctx context.Context, projectID uuid.UUID, currentLeadID, newLeadID string) error
	FindUserIDByEmail(ctx context.Context, email string) (string, error)
}

// pgxRepository implements Repository using a pgx connection pool.
type pgxRepository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed team repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &pgxRepository{db: db}
}

var _ Repository = (*pgxRepository)(nil)

// AddMember adds or reactivates a member in a project workspace.
func (r *pgxRepository) AddMember(ctx context.Context, pm *ProjectMember) error {
	if pm.ID == uuid.Nil {
		pm.ID = uuid.New()
	}
	if pm.JoinedAt.IsZero() {
		pm.JoinedAt = time.Now().UTC()
	}
	if pm.Status == "" {
		pm.Status = "active"
	}

	query := `
		INSERT INTO project_members (
			id, project_id, member_id, role, status, joined_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, member_id) DO UPDATE
		SET role = EXCLUDED.role,
		    status = 'active',
		    departed_at = NULL,
		    joined_at = EXCLUDED.joined_at
		RETURNING id, joined_at
	`
	err := r.db.QueryRow(ctx, query,
		pm.ID,
		pm.ProjectID,
		pm.MemberID,
		string(pm.Role),
		pm.Status,
		pm.JoinedAt,
	).Scan(&pm.ID, &pm.JoinedAt)
	if err != nil {
		return fmt.Errorf("failed to add project member: %w", err)
	}
	return nil
}

// GetMember retrieves a single active member from a project with enriched profile details.
func (r *pgxRepository) GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*ProjectMember, error) {
	query := `
		SELECT
			pm.id, pm.project_id, pm.member_id, pm.role, pm.status, pm.joined_at, pm.departed_at,
			COALESCE(u.email, ''),
			COALESCE(p.first_name, ''),
			COALESCE(p.last_name, ''),
			COALESCE(p.department, '')
		FROM project_members pm
		LEFT JOIN users u ON pm.member_id = u.id
		LEFT JOIN profiles p ON pm.member_id = p.user_id
		WHERE pm.project_id = $1 AND pm.member_id = $2 AND pm.status = 'active'
	`
	var pm ProjectMember
	var roleStr string
	err := r.db.QueryRow(ctx, query, projectID, memberID).Scan(
		&pm.ID,
		&pm.ProjectID,
		&pm.MemberID,
		&roleStr,
		&pm.Status,
		&pm.JoinedAt,
		&pm.DepartedAt,
		&pm.Email,
		&pm.FirstName,
		&pm.LastName,
		&pm.Department,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to get project member: %w", err)
	}
	pm.Role = Role(roleStr)
	return &pm, nil
}

// ListMembers retrieves all active members belonging to a project.
func (r *pgxRepository) ListMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error) {
	query := `
		SELECT
			pm.id, pm.project_id, pm.member_id, pm.role, pm.status, pm.joined_at, pm.departed_at,
			COALESCE(u.email, ''),
			COALESCE(p.first_name, ''),
			COALESCE(p.last_name, ''),
			COALESCE(p.department, '')
		FROM project_members pm
		LEFT JOIN users u ON pm.member_id = u.id
		LEFT JOIN profiles p ON pm.member_id = p.user_id
		WHERE pm.project_id = $1 AND pm.status = 'active'
		ORDER BY
			CASE pm.role
				WHEN 'lead' THEN 1
				WHEN 'maintainer' THEN 2
				WHEN 'member' THEN 3
				WHEN 'viewer' THEN 4
				ELSE 5
			END,
			pm.joined_at ASC
	`
	rows, err := r.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list project members: %w", err)
	}
	defer rows.Close()

	members := make([]ProjectMember, 0)
	for rows.Next() {
		var pm ProjectMember
		var roleStr string
		if err := rows.Scan(
			&pm.ID,
			&pm.ProjectID,
			&pm.MemberID,
			&roleStr,
			&pm.Status,
			&pm.JoinedAt,
			&pm.DepartedAt,
			&pm.Email,
			&pm.FirstName,
			&pm.LastName,
			&pm.Department,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project member: %w", err)
		}
		pm.Role = Role(roleStr)
		members = append(members, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error listing project members: %w", err)
	}
	return members, nil
}

// UpdateRole updates the role of an active project member.
func (r *pgxRepository) UpdateRole(ctx context.Context, projectID uuid.UUID, memberID string, role Role) error {
	query := `
		UPDATE project_members
		SET role = $1
		WHERE project_id = $2 AND member_id = $3 AND status = 'active'
	`
	tag, err := r.db.Exec(ctx, query, string(role), projectID, memberID)
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// RemoveMember marks a project member as departed.
func (r *pgxRepository) RemoveMember(ctx context.Context, projectID uuid.UUID, memberID string) error {
	query := `
		UPDATE project_members
		SET status = 'departed', departed_at = NOW()
		WHERE project_id = $1 AND member_id = $2 AND status = 'active'
	`
	tag, err := r.db.Exec(ctx, query, projectID, memberID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// CreateInvitation persists a new invitation to join a project workspace.
func (r *pgxRepository) CreateInvitation(ctx context.Context, inv *ProjectInvitation) error {
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	now := time.Now().UTC()
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = now
	}
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = now.Add(14 * 24 * time.Hour)
	}
	if inv.Status == "" {
		inv.Status = "pending"
	}

	query := `
		INSERT INTO project_invitations (
			id, project_id, inviter_id, invitee_id, proposed_role, status, expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
	`
	_, err := r.db.Exec(ctx, query,
		inv.ID,
		inv.ProjectID,
		inv.InviterID,
		inv.InviteeID,
		string(inv.ProposedRole),
		inv.Status,
		inv.ExpiresAt,
		inv.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// If duplicate, try to renew if prior invite was declined or expired
			renewQuery := `
				UPDATE project_invitations
				SET id = $1, inviter_id = $3, proposed_role = $4, status = 'pending', expires_at = $5, updated_at = NOW()
				WHERE project_id = $2 AND invitee_id = $6 AND status IN ('declined', 'expired')
			`
			tag, renewErr := r.db.Exec(ctx, renewQuery,
				inv.ID,
				inv.ProjectID,
				inv.InviterID,
				string(inv.ProposedRole),
				inv.ExpiresAt,
				inv.InviteeID,
			)
			if renewErr == nil && tag.RowsAffected() > 0 {
				return nil
			}
			return ErrDuplicateInvitation
		}
		return fmt.Errorf("failed to create invitation: %w", err)
	}
	return nil
}

// GetInvitation retrieves a single invitation by its UUID.
func (r *pgxRepository) GetInvitation(ctx context.Context, id uuid.UUID) (*ProjectInvitation, error) {
	query := `
		SELECT id, project_id, inviter_id, invitee_id, proposed_role, status, expires_at, created_at
		FROM project_invitations
		WHERE id = $1
	`
	var inv ProjectInvitation
	var roleStr string
	err := r.db.QueryRow(ctx, query, id).Scan(
		&inv.ID,
		&inv.ProjectID,
		&inv.InviterID,
		&inv.InviteeID,
		&roleStr,
		&inv.Status,
		&inv.ExpiresAt,
		&inv.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvitationNotFound
		}
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}
	inv.ProposedRole = Role(roleStr)
	return &inv, nil
}

// UpdateInvitationStatus updates the status of an invitation.
func (r *pgxRepository) UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `
		UPDATE project_invitations
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	tag, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update invitation status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

// CountLeads counts active leads for a project.
func (r *pgxRepository) CountLeads(ctx context.Context, projectID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM project_members
		WHERE project_id = $1 AND role = 'lead' AND status = 'active'
	`
	var count int
	err := r.db.QueryRow(ctx, query, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count project leads: %w", err)
	}
	return count, nil
}

// TransferLeadershipTx demotes the current lead, promotes the new lead, and updates project ownership.
func (r *pgxRepository) TransferLeadershipTx(ctx context.Context, projectID uuid.UUID, currentLeadID, newLeadID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 1. Demote current lead to maintainer
	demoteQuery := `
		UPDATE project_members
		SET role = 'maintainer'
		WHERE project_id = $1 AND member_id = $2 AND role = 'lead' AND status = 'active'
	`
	demoteTag, err := tx.Exec(ctx, demoteQuery, projectID, currentLeadID)
	if err != nil {
		return fmt.Errorf("failed to demote current lead: %w", err)
	}
	if demoteTag.RowsAffected() == 0 {
		return ErrUnauthorizedAction
	}

	// 2. Promote target to lead
	promoteQuery := `
		UPDATE project_members
		SET role = 'lead'
		WHERE project_id = $1 AND member_id = $2 AND status = 'active'
	`
	promoteTag, err := tx.Exec(ctx, promoteQuery, projectID, newLeadID)
	if err != nil {
		return fmt.Errorf("failed to promote new lead: %w", err)
	}
	if promoteTag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}

	// 3. Update project owner
	ownerQuery := `
		UPDATE projects
		SET owner_member_id = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err = tx.Exec(ctx, ownerQuery, newLeadID, projectID)
	if err != nil {
		return fmt.Errorf("failed to update project owner: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit leadership transfer: %w", err)
	}
	return nil
}

// FindUserIDByEmail looks up a user's UUID string by institutional email.
func (r *pgxRepository) FindUserIDByEmail(ctx context.Context, email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", ErrUserNotFound
	}
	var id string
	query := `SELECT id::text FROM users WHERE LOWER(email) = LOWER($1);`
	err := r.db.QueryRow(ctx, query, email).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("failed to find user by email: %w", err)
	}
	return id, nil
}
