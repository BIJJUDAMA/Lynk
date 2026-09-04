package user

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository defines the persistence interface for users and profiles.
type UserRepository interface {
	UpsertUser(ctx context.Context, u *User) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetStudentProfile(ctx context.Context, userID uuid.UUID) (*StudentProfile, error)
	GetStudentProfileByID(ctx context.Context, id uuid.UUID) (*StudentProfile, error)
	UpsertStudentProfile(ctx context.Context, sp *StudentProfile) error
	UpdateStudentResume(ctx context.Context, userID uuid.UUID, key, filename string, size int64) error
	GetEmployerProfile(ctx context.Context, userID uuid.UUID) (*EmployerProfile, error)
	UpsertEmployerProfile(ctx context.Context, ep *EmployerProfile) error
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

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
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

func (r *Repository) GetStudentProfile(ctx context.Context, userID uuid.UUID) (*StudentProfile, error) {
	sp := &StudentProfile{}
	var linksJSON []byte
	query := `
		SELECT id, user_id, first_name, last_name, bio, department, graduation_year, skills,
		       portfolio_links, resume_key, resume_filename, resume_byte_size, updated_at
		FROM student_profiles WHERE user_id = $1;
	`
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&sp.ID, &sp.UserID, &sp.FirstName, &sp.LastName, &sp.Bio, &sp.Department,
		&sp.GraduationYear, &sp.Skills, &linksJSON, &sp.ResumeKey, &sp.ResumeFilename,
		&sp.ResumeByteSize, &sp.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(linksJSON) > 0 {
		_ = json.Unmarshal(linksJSON, &sp.PortfolioLinks)
	}
	if sp.Skills == nil {
		sp.Skills = []string{}
	}
	if sp.PortfolioLinks == nil {
		sp.PortfolioLinks = []string{}
	}
	return sp, nil
}

func (r *Repository) GetStudentProfileByID(ctx context.Context, id uuid.UUID) (*StudentProfile, error) {
	sp := &StudentProfile{}
	var linksJSON []byte
	query := `
		SELECT id, user_id, first_name, last_name, bio, department, graduation_year, skills,
		       portfolio_links, resume_key, resume_filename, resume_byte_size, updated_at
		FROM student_profiles WHERE id = $1 OR user_id = $1;
	`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&sp.ID, &sp.UserID, &sp.FirstName, &sp.LastName, &sp.Bio, &sp.Department,
		&sp.GraduationYear, &sp.Skills, &linksJSON, &sp.ResumeKey, &sp.ResumeFilename,
		&sp.ResumeByteSize, &sp.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(linksJSON) > 0 {
		_ = json.Unmarshal(linksJSON, &sp.PortfolioLinks)
	}
	if sp.Skills == nil {
		sp.Skills = []string{}
	}
	if sp.PortfolioLinks == nil {
		sp.PortfolioLinks = []string{}
	}
	return sp, nil
}

func (r *Repository) UpsertStudentProfile(ctx context.Context, sp *StudentProfile) error {
	if sp.Skills == nil {
		sp.Skills = []string{}
	}
	if sp.PortfolioLinks == nil {
		sp.PortfolioLinks = []string{}
	}
	linksJSON, err := json.Marshal(sp.PortfolioLinks)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO student_profiles (user_id, first_name, last_name, bio, department, graduation_year, skills, portfolio_links, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name, bio = EXCLUDED.bio,
		    department = EXCLUDED.department, graduation_year = EXCLUDED.graduation_year,
		    skills = EXCLUDED.skills, portfolio_links = EXCLUDED.portfolio_links, updated_at = NOW()
		RETURNING id, user_id, first_name, last_name, bio, department, graduation_year, skills,
		          portfolio_links, resume_key, resume_filename, resume_byte_size, updated_at;
	`
	var returnedLinks []byte
	err = r.db.QueryRow(ctx, query, sp.UserID, sp.FirstName, sp.LastName, sp.Bio, sp.Department, sp.GraduationYear, sp.Skills, linksJSON).Scan(
		&sp.ID, &sp.UserID, &sp.FirstName, &sp.LastName, &sp.Bio, &sp.Department,
		&sp.GraduationYear, &sp.Skills, &returnedLinks, &sp.ResumeKey, &sp.ResumeFilename,
		&sp.ResumeByteSize, &sp.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if len(returnedLinks) > 0 {
		_ = json.Unmarshal(returnedLinks, &sp.PortfolioLinks)
	}
	if sp.Skills == nil {
		sp.Skills = []string{}
	}
	if sp.PortfolioLinks == nil {
		sp.PortfolioLinks = []string{}
	}
	return nil
}

func (r *Repository) UpdateStudentResume(ctx context.Context, userID uuid.UUID, key, filename string, size int64) error {
	query := `
		UPDATE student_profiles
		SET resume_key = $1, resume_filename = $2, resume_byte_size = $3, updated_at = NOW()
		WHERE user_id = $4;
	`
	tag, err := r.db.Exec(ctx, query, key, filename, size, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		insertQuery := `
			INSERT INTO student_profiles (user_id, resume_key, resume_filename, resume_byte_size, updated_at)
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

func (r *Repository) GetEmployerProfile(ctx context.Context, userID uuid.UUID) (*EmployerProfile, error) {
	ep := &EmployerProfile{}
	query := `
		SELECT id, user_id, company_or_org, contact_name, description, website, updated_at
		FROM employer_profiles WHERE user_id = $1;
	`
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&ep.ID, &ep.UserID, &ep.CompanyOrOrg, &ep.ContactName, &ep.Description, &ep.Website, &ep.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ep, nil
}

func (r *Repository) UpsertEmployerProfile(ctx context.Context, ep *EmployerProfile) error {
	query := `
		INSERT INTO employer_profiles (user_id, company_or_org, contact_name, description, website, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET company_or_org = EXCLUDED.company_or_org, contact_name = EXCLUDED.contact_name,
		    description = EXCLUDED.description, website = EXCLUDED.website, updated_at = NOW()
		RETURNING id, user_id, company_or_org, contact_name, description, website, updated_at;
	`
	return r.db.QueryRow(ctx, query, ep.UserID, ep.CompanyOrOrg, ep.ContactName, ep.Description, ep.Website).Scan(
		&ep.ID, &ep.UserID, &ep.CompanyOrOrg, &ep.ContactName, &ep.Description, &ep.Website, &ep.UpdatedAt,
	)
}
