package user

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository defines the persistence interface for users and unified profiles.
type UserRepository interface {
	UpsertUser(ctx context.Context, u *User) error
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetProfile(ctx context.Context, userID string) (*Profile, error)
	GetProfileByID(ctx context.Context, id string) (*Profile, error)
	UpsertProfile(ctx context.Context, p *Profile) error
	UpdateResume(ctx context.Context, userID string, key, filename string, size int64) error
}

// Repository implements UserRepository using pgxpool.Pool against PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new Repository backed by PostgreSQL.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ UserRepository = (*Repository)(nil)

func (r *Repository) UpsertUser(ctx context.Context, u *User) error {
	query := `
		INSERT INTO users (id, email, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET email = EXCLUDED.email, role = EXCLUDED.role, updated_at = NOW()
		RETURNING id, email, role, created_at, updated_at;
	`
	return r.db.QueryRow(ctx, query, u.ID, u.Email, u.Role).Scan(
		&u.ID, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	query := `SELECT id, email, role, created_at, updated_at FROM users WHERE id = $1;`
	err := r.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	p := &Profile{}
	var linksJSON []byte
	query := `
		SELECT id, user_id, first_name, last_name, bio, department, graduation_year, skills,
		       portfolio_links, resume_key, resume_filename, resume_byte_size,
		       organization, organization_website, updated_at
		FROM profiles WHERE user_id = $1;
	`
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&p.ID, &p.UserID, &p.FirstName, &p.LastName, &p.Bio, &p.Department,
		&p.GraduationYear, &p.Skills, &linksJSON, &p.ResumeKey, &p.ResumeFilename,
		&p.ResumeByteSize, &p.Organization, &p.OrganizationWebsite, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(linksJSON) > 0 {
		_ = json.Unmarshal(linksJSON, &p.PortfolioLinks)
	}
	if p.Skills == nil {
		p.Skills = []string{}
	}
	if p.PortfolioLinks == nil {
		p.PortfolioLinks = []string{}
	}
	return p, nil
}

// BuildGetProfileByIDQuery constructs a SARGable query and argument list for profile lookup by ID or UserID.
func BuildGetProfileByIDQuery(id string) (string, []interface{}) {
	if parsedUUID, err := uuid.Parse(id); err == nil {
		query := `
		SELECT id, user_id, first_name, last_name, bio, department, graduation_year, skills,
		       portfolio_links, resume_key, resume_filename, resume_byte_size,
		       organization, organization_website, updated_at
		FROM profiles WHERE id = $1 OR user_id = $2;
	`
		return query, []interface{}{parsedUUID, id}
	}

	query := `
		SELECT id, user_id, first_name, last_name, bio, department, graduation_year, skills,
		       portfolio_links, resume_key, resume_filename, resume_byte_size,
		       organization, organization_website, updated_at
		FROM profiles WHERE user_id = $1;
	`
	return query, []interface{}{id}
}

func (r *Repository) GetProfileByID(ctx context.Context, id string) (*Profile, error) {
	p := &Profile{}
	var linksJSON []byte
	query, args := BuildGetProfileByIDQuery(id)
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&p.ID, &p.UserID, &p.FirstName, &p.LastName, &p.Bio, &p.Department,
		&p.GraduationYear, &p.Skills, &linksJSON, &p.ResumeKey, &p.ResumeFilename,
		&p.ResumeByteSize, &p.Organization, &p.OrganizationWebsite, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(linksJSON) > 0 {
		_ = json.Unmarshal(linksJSON, &p.PortfolioLinks)
	}
	if p.Skills == nil {
		p.Skills = []string{}
	}
	if p.PortfolioLinks == nil {
		p.PortfolioLinks = []string{}
	}
	return p, nil
}

func (r *Repository) UpsertProfile(ctx context.Context, p *Profile) error {
	if p.Skills == nil {
		p.Skills = []string{}
	}
	if p.PortfolioLinks == nil {
		p.PortfolioLinks = []string{}
	}
	linksJSON, err := json.Marshal(p.PortfolioLinks)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO profiles (user_id, first_name, last_name, bio, department, graduation_year, skills, portfolio_links, organization, organization_website, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name, bio = EXCLUDED.bio,
		    department = EXCLUDED.department, graduation_year = EXCLUDED.graduation_year,
		    skills = EXCLUDED.skills, portfolio_links = EXCLUDED.portfolio_links,
		    organization = EXCLUDED.organization, organization_website = EXCLUDED.organization_website,
		    updated_at = NOW()
		RETURNING id, user_id, first_name, last_name, bio, department, graduation_year, skills,
		          portfolio_links, resume_key, resume_filename, resume_byte_size, organization, organization_website, updated_at;
	`
	var returnedLinks []byte
	err = r.db.QueryRow(ctx, query, p.UserID, p.FirstName, p.LastName, p.Bio, p.Department, p.GraduationYear, p.Skills, linksJSON, p.Organization, p.OrganizationWebsite).Scan(
		&p.ID, &p.UserID, &p.FirstName, &p.LastName, &p.Bio, &p.Department,
		&p.GraduationYear, &p.Skills, &returnedLinks, &p.ResumeKey, &p.ResumeFilename,
		&p.ResumeByteSize, &p.Organization, &p.OrganizationWebsite, &p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if len(returnedLinks) > 0 {
		_ = json.Unmarshal(returnedLinks, &p.PortfolioLinks)
	}
	if p.Skills == nil {
		p.Skills = []string{}
	}
	if p.PortfolioLinks == nil {
		p.PortfolioLinks = []string{}
	}
	return nil
}

func (r *Repository) UpdateResume(ctx context.Context, userID string, key, filename string, size int64) error {
	query := `
		UPDATE profiles
		SET resume_key = $1, resume_filename = $2, resume_byte_size = $3, updated_at = NOW()
		WHERE user_id = $4;
	`
	tag, err := r.db.Exec(ctx, query, key, filename, size, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		insertQuery := `
			INSERT INTO profiles (user_id, resume_key, resume_filename, resume_byte_size, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (user_id) DO UPDATE
			SET resume_key = EXCLUDED.resume_key,
			    resume_filename = EXCLUDED.resume_filename,
			    resume_byte_size = EXCLUDED.resume_byte_size,
			    updated_at = NOW();
		`
		_, err = r.db.Exec(ctx, insertQuery, userID, key, filename, size)
		return err
	}
	return nil
}
