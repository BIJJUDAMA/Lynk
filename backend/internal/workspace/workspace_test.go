package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httpx"
	"github.com/lynk/backend/internal/project"
	"github.com/lynk/backend/internal/team"
	"github.com/lynk/backend/internal/workspace"
)

// Mock workspace repository for unit and handler testing.
type mockWorkspaceRepo struct {
	sections   map[uuid.UUID][]workspace.ProjectSection
	resources  map[uuid.UUID]map[uuid.UUID]*workspace.ProjectResource
	activities map[uuid.UUID][]workspace.ProjectActivity
}

func newMockWorkspaceRepo() *mockWorkspaceRepo {
	return &mockWorkspaceRepo{
		sections:   make(map[uuid.UUID][]workspace.ProjectSection),
		resources:  make(map[uuid.UUID]map[uuid.UUID]*workspace.ProjectResource),
		activities: make(map[uuid.UUID][]workspace.ProjectActivity),
	}
}

func (m *mockWorkspaceRepo) GetSections(ctx context.Context, projectID uuid.UUID) ([]workspace.ProjectSection, error) {
	if s, ok := m.sections[projectID]; ok {
		return s, nil
	}
	return []workspace.ProjectSection{}, nil
}

func (m *mockWorkspaceRepo) UpsertSections(ctx context.Context, projectID uuid.UUID, sections []workspace.ProjectSection) error {
	m.sections[projectID] = sections
	return nil
}

func (m *mockWorkspaceRepo) ListResources(ctx context.Context, projectID uuid.UUID) ([]workspace.ProjectResource, error) {
	var list []workspace.ProjectResource
	if resMap, ok := m.resources[projectID]; ok {
		for _, r := range resMap {
			list = append(list, *r)
		}
	}
	return list, nil
}

func (m *mockWorkspaceRepo) AddResource(ctx context.Context, res *workspace.ProjectResource) error {
	if m.resources[res.ProjectID] == nil {
		m.resources[res.ProjectID] = make(map[uuid.UUID]*workspace.ProjectResource)
	}
	m.resources[res.ProjectID][res.ID] = res
	return nil
}

func (m *mockWorkspaceRepo) DeleteResource(ctx context.Context, projectID, resourceID uuid.UUID) error {
	resMap, ok := m.resources[projectID]
	if !ok {
		return workspace.ErrResourceNotFound
	}
	if _, exists := resMap[resourceID]; !exists {
		return workspace.ErrResourceNotFound
	}
	delete(resMap, resourceID)
	return nil
}

func (m *mockWorkspaceRepo) RecordActivity(ctx context.Context, act *workspace.ProjectActivity) error {
	m.activities[act.ProjectID] = append([]workspace.ProjectActivity{*act}, m.activities[act.ProjectID]...)
	return nil
}

func (m *mockWorkspaceRepo) ListActivity(ctx context.Context, projectID uuid.UUID, limit int) ([]workspace.ProjectActivity, error) {
	acts := m.activities[projectID]
	if limit > 0 && len(acts) > limit {
		return acts[:limit], nil
	}
	return acts, nil
}

// Mock project reader.
type mockProjectReader struct {
	projects map[uuid.UUID]*project.Project
}

func (m *mockProjectReader) GetProjectByID(ctx context.Context, id uuid.UUID) (*project.Project, error) {
	if p, ok := m.projects[id]; ok {
		return p, nil
	}
	return nil, project.ErrProjectNotFound
}

// Mock membership reader.
type mockMembershipReader struct {
	members map[string]*team.ProjectMember
}

func (m *mockMembershipReader) GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*team.ProjectMember, error) {
	key := projectID.String() + ":" + memberID
	if mem, ok := m.members[key]; ok {
		return mem, nil
	}
	return nil, team.ErrMemberNotFound
}

// Test SectionRegistry validation rules.
func TestSectionRegistry_Validation(t *testing.T) {
	registry := workspace.NewSectionRegistry()

	// Valid section
	valid := workspace.SectionConfig{
		Type:     "resources",
		Enabled:  true,
		Position: 2,
		Title:    "Project Links",
	}
	if err := registry.Validate(valid); err != nil {
		t.Fatalf("expected valid section to pass: %v", err)
	}

	// Invalid section type (arbitrary executable component attempt)
	invalid := workspace.SectionConfig{
		Type:     "custom_script_runner",
		Enabled:  true,
		Position: 3,
		Title:    "Custom Script",
	}
	if err := registry.Validate(invalid); err == nil {
		t.Fatalf("expected arbitrary section type to fail validation")
	}

	// Negative position check
	negativePos := workspace.SectionConfig{
		Type:     "overview",
		Enabled:  true,
		Position: -1,
		Title:    "Overview",
	}
	if err := registry.Validate(negativePos); err != workspace.ErrInvalidPosition {
		t.Fatalf("expected ErrInvalidPosition, got %v", err)
	}
}

// Test SectionRegistry batch validation and defaults.
func TestSectionRegistry_ValidateSections(t *testing.T) {
	registry := workspace.NewSectionRegistry()
	projectID := uuid.New()

	defaults := registry.DefaultSections(projectID)
	if len(defaults) != 6 {
		t.Fatalf("expected 6 default sections, got %d", len(defaults))
	}

	if err := registry.ValidateSections(defaults); err != nil {
		t.Fatalf("expected default sections to validate cleanly: %v", err)
	}

	// Duplicate section type
	duplicate := []workspace.ProjectSection{
		{
			ProjectID:   projectID,
			SectionType: workspace.SectionTypeOverview,
			Title:       "Overview 1",
			Position:    0,
		},
		{
			ProjectID:   projectID,
			SectionType: workspace.SectionTypeOverview,
			Title:       "Overview 2",
			Position:    1,
		},
	}
	if err := registry.ValidateSections(duplicate); err == nil {
		t.Fatalf("expected duplicate section types to fail validation")
	}

	// Empty title
	emptyTitle := []workspace.ProjectSection{
		{
			ProjectID:   projectID,
			SectionType: workspace.SectionTypeTeam,
			Title:       "   ",
			Position:    0,
		},
	}
	if err := registry.ValidateSections(emptyTitle); err == nil {
		t.Fatalf("expected empty title to fail validation")
	}
}

// Test WorkspaceService permissions and role checks.
func TestWorkspaceService_SectionsPermissions(t *testing.T) {
	repo := newMockWorkspaceRepo()
	projectID := uuid.New()
	ownerID := "user-owner"
	leadID := "user-lead"
	maintID := "user-maint"
	memberID := "user-member"
	outsiderID := "user-outsider"

	projReader := &mockProjectReader{
		projects: map[uuid.UUID]*project.Project{
			projectID: {
				ID:            projectID,
				OwnerMemberID: ownerID,
				Visibility:    project.VisibilityPublic,
			},
		},
	}

	memReader := &mockMembershipReader{
		members: map[string]*team.ProjectMember{
			projectID.String() + ":" + leadID: {
				ProjectID: projectID,
				MemberID:  leadID,
				Role:      team.RoleLead,
				Status:    "active",
			},
			projectID.String() + ":" + maintID: {
				ProjectID: projectID,
				MemberID:  maintID,
				Role:      team.RoleMaintainer,
				Status:    "active",
			},
			projectID.String() + ":" + memberID: {
				ProjectID: projectID,
				MemberID:  memberID,
				Role:      team.RoleMember,
				Status:    "active",
			},
		},
	}

	svc := workspace.NewService(repo, projReader, memReader)
	ctx := context.Background()

	testSections := []workspace.ProjectSection{
		{
			ProjectID:   projectID,
			SectionType: workspace.SectionTypeOverview,
			Title:       "Updated Overview",
			Position:    0,
			Enabled:     true,
		},
	}

	// 1. Lead can update sections
	if err := svc.UpdateSections(ctx, projectID, leadID, testSections); err != nil {
		t.Fatalf("lead should be able to update sections: %v", err)
	}

	// Verify activity was recorded
	acts, err := svc.ListActivity(ctx, projectID, leadID, 10)
	if err != nil || len(acts) != 1 || acts[0].EventType != "sections_updated" {
		t.Fatalf("expected activity for sections_updated, got %v", acts)
	}

	// 2. Maintainer can update sections
	if err := svc.UpdateSections(ctx, projectID, maintID, testSections); err != nil {
		t.Fatalf("maintainer should be able to update sections: %v", err)
	}

	// 3. Regular member cannot update sections
	if err := svc.UpdateSections(ctx, projectID, memberID, testSections); err != workspace.ErrUnauthorizedWorkspaceAction {
		t.Fatalf("expected ErrUnauthorizedWorkspaceAction for regular member, got %v", err)
	}

	// 4. Outsider cannot update sections
	if err := svc.UpdateSections(ctx, projectID, outsiderID, testSections); err != workspace.ErrUnauthorizedWorkspaceAction {
		t.Fatalf("expected ErrUnauthorizedWorkspaceAction for outsider, got %v", err)
	}
}

// Test WorkspaceService visibility enforcement.
func TestWorkspaceService_Visibility(t *testing.T) {
	repo := newMockWorkspaceRepo()
	publicProjID := uuid.New()
	privateProjID := uuid.New()
	ownerID := "owner-1"
	memberID := "member-1"
	outsiderID := "outsider-1"

	projReader := &mockProjectReader{
		projects: map[uuid.UUID]*project.Project{
			publicProjID: {
				ID:            publicProjID,
				OwnerMemberID: ownerID,
				Visibility:    project.VisibilityPublic,
			},
			privateProjID: {
				ID:            privateProjID,
				OwnerMemberID: ownerID,
				Visibility:    project.VisibilityPrivate,
			},
		},
	}

	memReader := &mockMembershipReader{
		members: map[string]*team.ProjectMember{
			privateProjID.String() + ":" + memberID: {
				ProjectID: privateProjID,
				MemberID:  memberID,
				Role:      team.RoleMember,
				Status:    "active",
			},
		},
	}

	svc := workspace.NewService(repo, projReader, memReader)
	ctx := context.Background()

	// 1. Public project is viewable by anonymous user (empty userID)
	sections, err := svc.GetSections(ctx, publicProjID, "")
	if err != nil || len(sections) == 0 {
		t.Fatalf("public project should be viewable anonymously: %v", err)
	}

	// 2. Private project viewable by member
	sections, err = svc.GetSections(ctx, privateProjID, memberID)
	if err != nil || len(sections) == 0 {
		t.Fatalf("private project should be viewable by member: %v", err)
	}

	// 3. Private project rejected for outsider
	_, err = svc.GetSections(ctx, privateProjID, outsiderID)
	if err != workspace.ErrUnauthorizedWorkspaceAction {
		t.Fatalf("expected ErrUnauthorizedWorkspaceAction for outsider on private project, got %v", err)
	}

	// 4. Private project rejected for anonymous user
	_, err = svc.GetSections(ctx, privateProjID, "")
	if err != workspace.ErrUnauthorizedWorkspaceAction {
		t.Fatalf("expected ErrUnauthorizedWorkspaceAction for anonymous on private project, got %v", err)
	}

	// 5. Non-existent project
	_, err = svc.GetSections(ctx, uuid.New(), "")
	if err != project.ErrProjectNotFound {
		t.Fatalf("expected ErrProjectNotFound, got %v", err)
	}
}

// Test external resource operations and URL validation.
func TestWorkspaceService_Resources(t *testing.T) {
	repo := newMockWorkspaceRepo()
	projectID := uuid.New()
	leadID := "lead-1"
	memberID := "member-1"

	projReader := &mockProjectReader{
		projects: map[uuid.UUID]*project.Project{
			projectID: {
				ID:            projectID,
				OwnerMemberID: leadID,
				Visibility:    project.VisibilityPublic,
			},
		},
	}

	memReader := &mockMembershipReader{
		members: map[string]*team.ProjectMember{
			projectID.String() + ":" + leadID: {
				ProjectID: projectID,
				MemberID:  leadID,
				Role:      team.RoleLead,
				Status:    "active",
			},
			projectID.String() + ":" + memberID: {
				ProjectID: projectID,
				MemberID:  memberID,
				Role:      team.RoleMember,
				Status:    "active",
			},
		},
	}

	svc := workspace.NewService(repo, projReader, memReader)
	ctx := context.Background()

	// 1. Add valid resource
	res, err := svc.AddResource(ctx, projectID, leadID, workspace.CreateResourceInput{
		Label:        "GitHub Repository",
		URL:          "https://github.com/lynk/workspace",
		ResourceType: workspace.ResourceTypeGitHub,
	})
	if err != nil {
		t.Fatalf("expected valid resource addition to succeed: %v", err)
	}
	if res.ID == uuid.Nil || res.Label != "GitHub Repository" {
		t.Fatalf("unexpected resource data: %+v", res)
	}

	// 2. Reject invalid URL schemes (javascript:, file:, ftp:)
	invalidURLs := []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"ftp://example.com/file",
		"https://",
		"",
	}
	for _, invURL := range invalidURLs {
		_, err := svc.AddResource(ctx, projectID, leadID, workspace.CreateResourceInput{
			Label: "Bad Link",
			URL:   invURL,
		})
		if err != workspace.ErrInvalidResourceURL {
			t.Errorf("expected ErrInvalidResourceURL for %q, got %v", invURL, err)
		}
	}

	// 3. Regular member cannot add resource
	_, err = svc.AddResource(ctx, projectID, memberID, workspace.CreateResourceInput{
		Label: "Demo Link",
		URL:   "https://demo.example.com",
	})
	if err != workspace.ErrUnauthorizedWorkspaceAction {
		t.Fatalf("expected ErrUnauthorizedWorkspaceAction for regular member, got %v", err)
	}

	// 4. Regular member cannot delete resource
	err = svc.DeleteResource(ctx, projectID, res.ID, memberID)
	if err != workspace.ErrUnauthorizedWorkspaceAction {
		t.Fatalf("expected ErrUnauthorizedWorkspaceAction for delete by regular member, got %v", err)
	}

	// 5. Lead can delete resource
	err = svc.DeleteResource(ctx, projectID, res.ID, leadID)
	if err != nil {
		t.Fatalf("lead should be able to delete resource: %v", err)
	}

	// 6. Delete already deleted resource returns ErrResourceNotFound
	err = svc.DeleteResource(ctx, projectID, res.ID, leadID)
	if err != workspace.ErrResourceNotFound {
		t.Fatalf("expected ErrResourceNotFound, got %v", err)
	}
}

// Test HTTP Handler endpoints.
func TestWorkspaceHandler_HTTP(t *testing.T) {
	repo := newMockWorkspaceRepo()
	projectID := uuid.New()
	leadID := "lead-http-user"
	memberID := "member-http-user"

	projReader := &mockProjectReader{
		projects: map[uuid.UUID]*project.Project{
			projectID: {
				ID:            projectID,
				OwnerMemberID: leadID,
				Visibility:    project.VisibilityPublic,
			},
		},
	}

	memReader := &mockMembershipReader{
		members: map[string]*team.ProjectMember{
			projectID.String() + ":" + leadID: {
				ProjectID: projectID,
				MemberID:  leadID,
				Role:      team.RoleLead,
				Status:    "active",
			},
			projectID.String() + ":" + memberID: {
				ProjectID: projectID,
				MemberID:  memberID,
				Role:      team.RoleMember,
				Status:    "active",
			},
		},
	}

	svc := workspace.NewService(repo, projReader, memReader)
	h := workspace.NewHandler(svc)

	router := h.Routes(httpx.RequireAuthFromContext())

	// 1. GET /projects/{id}/sections (Public)
	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/sections", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET sections, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. PUT /projects/{id}/sections with Lead auth
	updatePayload := []workspace.ProjectSection{
		{
			ProjectID:   projectID,
			SectionType: workspace.SectionTypeOverview,
			Title:       "Customized Overview",
			Position:    0,
			Enabled:     true,
		},
	}
	body, _ := json.Marshal(updatePayload)
	req = httptest.NewRequest(http.MethodPut, "/projects/"+projectID.String()+"/sections", bytes.NewReader(body))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.UserClaims{UserID: leadID}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PUT sections, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. PUT /projects/{id}/sections with Member auth (403 Forbidden)
	req = httptest.NewRequest(http.MethodPut, "/projects/"+projectID.String()+"/sections", bytes.NewReader(body))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.UserClaims{UserID: memberID}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for member PUT sections, got %d", rec.Code)
	}

	// 4. POST /projects/{id}/resources with Lead auth (201 Created)
	resInput := workspace.CreateResourceInput{
		Label:        "Documentation",
		URL:          "https://docs.lynk.campus/handbook",
		ResourceType: workspace.ResourceTypeDocs,
	}
	resBody, _ := json.Marshal(resInput)
	req = httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/resources", bytes.NewReader(resBody))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.UserClaims{UserID: leadID}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST resource, got %d: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Data workspace.ProjectResource `json:"data"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&createResp)
	createdResourceID := createResp.Data.ID

	// 5. POST /projects/{id}/resources with invalid URL (400 Bad Request)
	badResInput := workspace.CreateResourceInput{
		Label: "Invalid",
		URL:   "not-a-valid-url",
	}
	badResBody, _ := json.Marshal(badResInput)
	req = httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/resources", bytes.NewReader(badResBody))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.UserClaims{UserID: leadID}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for bad URL, got %d", rec.Code)
	}

	// 6. GET /projects/{id}/resources (200 OK)
	req = httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/resources", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET resources, got %d", rec.Code)
	}

	// 7. DELETE /projects/{id}/resources/{resourceId} with Lead auth (200 OK)
	req = httptest.NewRequest(http.MethodDelete, "/projects/"+projectID.String()+"/resources/"+createdResourceID.String(), nil)
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.UserClaims{UserID: leadID}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for DELETE resource, got %d: %s", rec.Code, rec.Body.String())
	}

	// 8. GET /projects/{id}/activity (200 OK)
	req = httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/activity", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET activity, got %d: %s", rec.Code, rec.Body.String())
	}
}
