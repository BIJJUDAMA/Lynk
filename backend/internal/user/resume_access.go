package user

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ResumeAccessChecker authorizes job posters to download applicant resumes.
type ResumeAccessChecker interface {
	JobOwnerMayDownloadApplicantResume(ctx context.Context, ownerID, applicantUserID string) (bool, error)
}

// PGResumeAccessChecker checks job-owner/applicant relationships via PostgreSQL.
type PGResumeAccessChecker struct {
	db *pgxpool.Pool
}

// NewPGResumeAccessChecker creates a PostgreSQL-backed resume access checker.
func NewPGResumeAccessChecker(db *pgxpool.Pool) *PGResumeAccessChecker {
	return &PGResumeAccessChecker{db: db}
}

// JobOwnerMayDownloadApplicantResume returns true when ownerID posted a job that applicantUserID applied to.
func (c *PGResumeAccessChecker) JobOwnerMayDownloadApplicantResume(ctx context.Context, ownerID, applicantUserID string) (bool, error) {
	if c == nil || c.db == nil {
		return false, nil
	}
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM jobs j
			INNER JOIN applications a ON a.job_id = j.id
			WHERE j.created_by = $1
			  AND a.applicant_id = $2
		);
	`
	var ok bool
	if err := c.db.QueryRow(ctx, q, ownerID, applicantUserID).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}
