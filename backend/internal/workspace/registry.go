package workspace

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// SectionRegistry enforces platform-defined section types and guards against
// arbitrary executable components or unrecognized sections.
type SectionRegistry struct {
	allowedTypes map[string]bool
}

// NewSectionRegistry constructs a new section registry initialized with trusted section types.
func NewSectionRegistry() *SectionRegistry {
	return &SectionRegistry{
		allowedTypes: map[string]bool{
			SectionTypeOverview:    true,
			SectionTypeTeam:        true,
			SectionTypeResources:   true,
			SectionTypeRecruitment: true,
			SectionTypeUpdates:     true,
			SectionTypeDocs:        true,
		},
	}
}

// IsValidType checks if a given section type is in the trusted registry.
func (r *SectionRegistry) IsValidType(sectionType string) bool {
	return r.allowedTypes[sectionType]
}

// Validate verifies a single SectionConfig against platform rules.
func (r *SectionRegistry) Validate(s SectionConfig) error {
	if !r.IsValidType(s.Type) {
		return fmt.Errorf("%w: %q", ErrInvalidSectionType, s.Type)
	}
	if s.Position < 0 {
		return ErrInvalidPosition
	}
	return nil
}

// ValidateSection verifies a ProjectSection against registry rules.
func (r *SectionRegistry) ValidateSection(s ProjectSection) error {
	if !r.IsValidType(s.SectionType) {
		return fmt.Errorf("%w: %q", ErrInvalidSectionType, s.SectionType)
	}
	if s.Position < 0 {
		return ErrInvalidPosition
	}
	if strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("section title cannot be empty")
	}
	return nil
}

// ValidateSections validates an ordered collection of project sections, ensuring
// no duplicate section types, valid positions, and supported types.
func (r *SectionRegistry) ValidateSections(sections []ProjectSection) error {
	seen := make(map[string]bool)
	for _, s := range sections {
		if err := r.ValidateSection(s); err != nil {
			return err
		}
		if seen[s.SectionType] {
			return fmt.Errorf("%w: %q", ErrDuplicateSectionType, s.SectionType)
		}
		seen[s.SectionType] = true
	}
	return nil
}

// DefaultSections returns the standard default sections for a newly initialized project workspace.
func (r *SectionRegistry) DefaultSections(projectID uuid.UUID) []ProjectSection {
	return []ProjectSection{
		{
			ProjectID:   projectID,
			SectionType: SectionTypeOverview,
			Title:       "Overview",
			Enabled:     true,
			Position:    0,
			Config:      map[string]any{},
			Content:     "",
		},
		{
			ProjectID:   projectID,
			SectionType: SectionTypeTeam,
			Title:       "Team",
			Enabled:     true,
			Position:    1,
			Config:      map[string]any{},
			Content:     "",
		},
		{
			ProjectID:   projectID,
			SectionType: SectionTypeResources,
			Title:       "Resources",
			Enabled:     true,
			Position:    2,
			Config:      map[string]any{},
			Content:     "",
		},
		{
			ProjectID:   projectID,
			SectionType: SectionTypeRecruitment,
			Title:       "Recruitment",
			Enabled:     true,
			Position:    3,
			Config:      map[string]any{},
			Content:     "",
		},
		{
			ProjectID:   projectID,
			SectionType: SectionTypeUpdates,
			Title:       "Updates",
			Enabled:     true,
			Position:    4,
			Config:      map[string]any{},
			Content:     "",
		},
		{
			ProjectID:   projectID,
			SectionType: SectionTypeDocs,
			Title:       "Documentation",
			Enabled:     true,
			Position:    5,
			Config:      map[string]any{},
			Content:     "",
		},
	}
}
