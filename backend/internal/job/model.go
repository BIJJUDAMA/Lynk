package job

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Allowed job statuses
const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusClosed     = "closed"
	StatusCancelled  = "cancelled"
)

// CreatorInfo represents public campus member metadata attached to job postings.
type CreatorInfo struct {
	ID                  string `json:"id"`
	Email               string `json:"email"`
	FirstName           string `json:"first_name"`
	LastName            string `json:"last_name"`
	Department          string `json:"department"`
	Organization        string `json:"organization,omitempty"`
	OrganizationWebsite string `json:"organization_website,omitempty"`
}

// Job represents a job posting in the Lynk marketplace.
type Job struct {
	ID             uuid.UUID    `json:"id"`
	CreatedBy      string       `json:"created_by"`
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	RequiredSkills []string     `json:"required_skills"`
	Department     string       `json:"department"`
	Deadline       *time.Time   `json:"deadline,omitempty"`
	Status         string       `json:"status"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	Creator        *CreatorInfo `json:"creator,omitempty"`
}

// CreateJobRequest contains the payload required to post a new job.
type CreateJobRequest struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	RequiredSkills []string `json:"required_skills"`
	Department     string   `json:"department"`
	Deadline       *JobDate `json:"deadline,omitempty"`
}

// UpdateJobRequest contains optional fields to update an existing job posting.
type UpdateJobRequest struct {
	Title          *string   `json:"title,omitempty"`
	Description    *string   `json:"description,omitempty"`
	RequiredSkills *[]string `json:"required_skills,omitempty"`
	Department     *string   `json:"department,omitempty"`
	Deadline       *JobDate  `json:"deadline,omitempty"`
	Status         *string   `json:"status,omitempty"`
}

// JobFilter specifies criteria for searching and filtering job postings.
type JobFilter struct {
	Search     string   `json:"search,omitempty"`
	Department string   `json:"department,omitempty"`
	Skill      string   `json:"skill,omitempty"`
	Skills     []string `json:"skills,omitempty"`
	Status     string   `json:"status,omitempty"`
	CreatedBy  *string  `json:"created_by,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
}

// JobDate provides flexible date deserialization supporting ISO date (YYYY-MM-DD) and RFC3339.
type JobDate time.Time

// Time converts JobDate to *time.Time.
func (jd *JobDate) Time() *time.Time {
	if jd == nil {
		return nil
	}
	t := time.Time(*jd)
	if t.IsZero() {
		return nil
	}
	return &t
}

// UnmarshalJSON parses YYYY-MM-DD or RFC3339 date strings.
func (jd *JobDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "" || s == "null" {
		return nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		*jd = JobDate(t)
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		*jd = JobDate(t)
		return nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		*jd = JobDate(t)
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	*jd = JobDate(t)
	return nil
}

// MarshalJSON formats JobDate as YYYY-MM-DD string.
func (jd JobDate) MarshalJSON() ([]byte, error) {
	t := time.Time(jd)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Format("2006-01-02"))
}
