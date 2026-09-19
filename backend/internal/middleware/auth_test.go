package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/middleware"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword/epmodels"
	"github.com/supertokens/supertokens-golang/recipe/emailverification"
	"github.com/supertokens/supertokens-golang/recipe/emailverification/evmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/claims"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"github.com/supertokens/supertokens-golang/recipe/userroles/userrolesmodels"
	"github.com/supertokens/supertokens-golang/supertokens"
)

type mockValidator struct {
	claims *auth.UserClaims
	err    error
}

func (m *mockValidator) ValidateToken(ctx context.Context, token string) (*auth.UserClaims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

type errorResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestSessionMiddleware_MissingSession(t *testing.T) {
	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/protected", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Missing or invalid session credentials" {
		t.Errorf("expected message 'Missing or invalid session credentials', got %q", resp.Error.Message)
	}
}

func TestAuthMiddleware_DelegatesToSession(t *testing.T) {
	// Calling AuthMiddleware() with zero arguments should delegate to SessionMiddleware()
	handler := middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/protected", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Missing or invalid session credentials" {
		t.Errorf("expected message 'Missing or invalid session credentials', got %q", resp.Error.Message)
	}
}

func TestAuthMiddleware_Valid(t *testing.T) {
	v := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "user-123",
			Email:         "test@univ.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		},
	}

	handler := middleware.AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserContext(r.Context())
		if err != nil || claims.UserID != "user-123" {
			t.Errorf("unexpected claims: %v, err: %v", claims, err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	v := &mockValidator{}
	handler := middleware.AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	v := &mockValidator{}
	handler := middleware.AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Basic abcdef")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	v := &mockValidator{
		err: errors.New("signature is invalid"),
	}
	handler := middleware.AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "INVALID_TOKEN" {
		t.Errorf("expected code INVALID_TOKEN, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "signature is invalid" {
		t.Errorf("expected message 'signature is invalid', got %s", resp.Error.Message)
	}
}

func TestRequireRole_Admin(t *testing.T) {
	vAdmin := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "admin-123",
			Email:         "admin@univ.edu",
			EmailVerified: true,
			Roles:         []string{"admin"},
		},
	}

	// 1. User with "admin" role succeeds
	adminHandler := middleware.AuthMiddleware(vAdmin)(middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	adminHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK for admin role, got %d", rr.Code)
	}

	// 2. User without "admin" role is rejected (zero persona check, strict admin permission)
	vNonAdmin := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "user-456",
			Email:         "user@univ.edu",
			EmailVerified: true,
			Roles:         []string{},
		},
	}

	forbiddenHandler := middleware.AuthMiddleware(vNonAdmin)(middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	rr2 := httptest.NewRecorder()
	forbiddenHandler.ServeHTTP(rr2, req)

	if rr2.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-admin user, got %d", rr2.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "FORBIDDEN" {
		t.Errorf("expected code FORBIDDEN, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Insufficient permissions" {
		t.Errorf("expected message 'Insufficient permissions', got %q", resp.Error.Message)
	}
}

func TestRequireRole_NoContext(t *testing.T) {
	handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/admin", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden without context, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "FORBIDDEN" {
		t.Errorf("expected code FORBIDDEN, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Insufficient permissions" {
		t.Errorf("expected message 'Insufficient permissions', got %q", resp.Error.Message)
	}
}

func TestRequireVerifiedEmail_Forbidden(t *testing.T) {
	v := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "123",
			Email:         "test@univ.edu",
			EmailVerified: false,
			Roles:         []string{},
		},
	}

	chain := middleware.AuthMiddleware(v)(middleware.RequireVerifiedEmail()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/api/v1/jobs/1/applications", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()

	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for unverified email, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "EMAIL_NOT_VERIFIED" {
		t.Errorf("expected code EMAIL_NOT_VERIFIED, got %s", resp.Error.Code)
	}
	expectedMsg := "Campus verification pending: Please verify your institutional .edu email before accessing opportunities."
	if resp.Error.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, resp.Error.Message)
	}
}

func TestRequireVerifiedEmail_Success(t *testing.T) {
	v := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "123",
			Email:         "test@univ.edu",
			EmailVerified: true,
			Roles:         []string{},
		},
	}

	chain := middleware.AuthMiddleware(v)(middleware.RequireVerifiedEmail()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/api/v1/jobs/1/applications", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()

	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK for verified email, got %d", rr.Code)
	}
}

func TestRequireVerifiedEmail_NoContext(t *testing.T) {
	chain := middleware.RequireVerifiedEmail()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/jobs/1/applications", nil)
	rr := httptest.NewRecorder()

	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden without context, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "EMAIL_NOT_VERIFIED" {
		t.Errorf("expected code EMAIL_NOT_VERIFIED, got %s", resp.Error.Code)
	}
	expectedMsg := "Campus verification pending: Please verify your institutional .edu email before accessing opportunities."
	if resp.Error.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, resp.Error.Message)
	}
}

func initSuperTokensForMiddlewareTest(
	getSessionFn func() (sessmodels.SessionContainer, error),
	getUserByIDFn func(userID string) (*epmodels.User, error),
	isEmailVerifiedFn func(userID, email string) (bool, error),
	getRolesFn func(userID string) ([]string, error),
) error {
	supertokens.ResetForTest()
	apiBasePath := "/api/v1/auth"
	websiteBasePath := "/auth"

	return supertokens.Init(supertokens.TypeInput{
		Supertokens: &supertokens.ConnectionInfo{
			ConnectionURI: "http://localhost:3567",
		},
		AppInfo: supertokens.AppInfo{
			AppName:         "LynkTest",
			APIDomain:       "http://localhost:8080",
			WebsiteDomain:   "http://localhost:3000",
			APIBasePath:     &apiBasePath,
			WebsiteBasePath: &websiteBasePath,
		},
		RecipeList: []supertokens.Recipe{
			emailpassword.Init(&epmodels.TypeInput{
				Override: &epmodels.OverrideStruct{
					Functions: func(original epmodels.RecipeInterface) epmodels.RecipeInterface {
						if getUserByIDFn != nil {
							fn := func(userID string, userContext supertokens.UserContext) (*epmodels.User, error) {
								return getUserByIDFn(userID)
							}
							original.GetUserByID = &fn
						}
						return original
					},
				},
			}),
			emailverification.Init(evmodels.TypeInput{
				Mode: evmodels.ModeRequired,
				Override: &evmodels.OverrideStruct{
					Functions: func(original evmodels.RecipeInterface) evmodels.RecipeInterface {
						if isEmailVerifiedFn != nil {
							fn := func(userID, email string, userContext supertokens.UserContext) (bool, error) {
								return isEmailVerifiedFn(userID, email)
							}
							original.IsEmailVerified = &fn
						}
						return original
					},
				},
			}),
			session.Init(&sessmodels.TypeInput{
				Override: &sessmodels.OverrideStruct{
					Functions: func(original sessmodels.RecipeInterface) sessmodels.RecipeInterface {
						if getSessionFn != nil {
							fn := func(accessToken *string, antiCSRFToken *string, options *sessmodels.VerifySessionOptions, userContext supertokens.UserContext) (sessmodels.SessionContainer, error) {
								return getSessionFn()
							}
							original.GetSession = &fn
							globalClaimFn := func(userId string, claimValidatorsAddedByOtherRecipes []claims.SessionClaimValidator, tenantId string, userContext supertokens.UserContext) ([]claims.SessionClaimValidator, error) {
								return []claims.SessionClaimValidator{}, nil
							}
							original.GetGlobalClaimValidators = &globalClaimFn
						}
						return original
					},
				},
			}),
			userroles.Init(&userrolesmodels.TypeInput{
				Override: &userrolesmodels.OverrideStruct{
					Functions: func(original userrolesmodels.RecipeInterface) userrolesmodels.RecipeInterface {
						if getRolesFn != nil {
							fn := func(userID string, tenantId string, userContext supertokens.UserContext) (userrolesmodels.GetRolesForUserResponse, error) {
								roles, err := getRolesFn(userID)
								if err != nil {
									return userrolesmodels.GetRolesForUserResponse{}, err
								}
								return userrolesmodels.GetRolesForUserResponse{
									OK: &struct{ Roles []string }{Roles: roles},
								}, nil
							}
							original.GetRolesForUser = &fn
						}
						return original
					},
				},
			}),
		},
	})
}

func newMockSessionContainer(userID string, payload map[string]interface{}) sessmodels.SessionContainer {
	return &sessmodels.TypeSessionContainer{
		GetUserID:              func() string { return userID },
		GetUserIDWithContext:   func(userContext supertokens.UserContext) string { return userID },
		GetTenantId:            func() string { return "public" },
		GetTenantIdWithContext: func(userContext supertokens.UserContext) string { return "public" },
		GetAccessTokenPayload: func() map[string]interface{} {
			return payload
		},
		GetAccessTokenPayloadWithContext: func(userContext supertokens.UserContext) map[string]interface{} {
			return payload
		},
		AssertClaimsWithContext: func(claimValidators []claims.SessionClaimValidator, userContext supertokens.UserContext) error {
			return nil
		},
		AttachToRequestResponseWithContext: func(info sessmodels.RequestResponseInfo, userContext supertokens.UserContext) error {
			return nil
		},
	}
}

func TestClaimsFromAccessTokenPayload_PrefersTokenOverRPC(t *testing.T) {
	payload := map[string]interface{}{
		"email":         "a@stanford.edu",
		"emailVerified": true,
		"roles":         []interface{}{"member"},
	}
	email, verified, roles, ok := middleware.ParseSessionPayload(payload)
	if !ok || email != "a@stanford.edu" || !verified || len(roles) != 1 || roles[0] != "member" {
		t.Fatalf("ParseSessionPayload failed: %q %v %v ok=%v", email, verified, roles, ok)
	}
}

func TestSessionMiddleware_PrefersTokenClaimsOverRPC(t *testing.T) {
	rpcCalled := false
	err := initSuperTokensForMiddlewareTest(
		func() (sessmodels.SessionContainer, error) {
			return newMockSessionContainer("user_claims", map[string]interface{}{
				"email":         "a@stanford.edu",
				"emailVerified": true,
				"roles":         []interface{}{"member"},
			}), nil
		},
		func(userID string) (*epmodels.User, error) {
			rpcCalled = true
			return nil, errors.New("should not be called")
		},
		func(userID, email string) (bool, error) {
			rpcCalled = true
			return false, errors.New("should not be called")
		},
		func(userID string) ([]string, error) {
			rpcCalled = true
			return nil, errors.New("should not be called")
		},
	)
	if err != nil {
		t.Fatalf("failed to init supertokens: %v", err)
	}

	var capturedClaims *auth.UserClaims
	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserContext(r.Context())
		if err != nil {
			t.Fatalf("failed to get user context: %v", err)
		}
		capturedClaims = claims
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/profile/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
	if rpcCalled {
		t.Fatal("expected SuperTokens Core RPCs to be skipped when access-token claims are complete")
	}
	if capturedClaims == nil {
		t.Fatal("expected captured claims, got nil")
	}
	if capturedClaims.Email != "a@stanford.edu" || !capturedClaims.EmailVerified || len(capturedClaims.Roles) != 1 || capturedClaims.Roles[0] != "member" {
		t.Fatalf("unexpected claims: %+v", capturedClaims)
	}
}

func TestSessionMiddleware_HandlesMissingEmailInPayloadWithFallback(t *testing.T) {
	stUser := &epmodels.User{
		ID:    "st_user_fallback",
		Email: "fallback@stanford.edu",
	}

	err := initSuperTokensForMiddlewareTest(
		func() (sessmodels.SessionContainer, error) {
			return newMockSessionContainer("st_user_fallback", map[string]interface{}{}), nil
		},
		func(userID string) (*epmodels.User, error) {
			if userID == "st_user_fallback" {
				return stUser, nil
			}
			return nil, errors.New("user not found")
		},
		func(userID, email string) (bool, error) {
			return true, nil
		},
		func(userID string) ([]string, error) {
			return []string{"member"}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to init supertokens: %v", err)
	}

	var capturedClaims *auth.UserClaims
	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserContext(r.Context())
		if err != nil {
			t.Fatalf("failed to get user context: %v", err)
		}
		capturedClaims = claims
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/profile/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedClaims == nil {
		t.Fatal("expected captured claims, got nil")
	}
	if capturedClaims.Email != "fallback@stanford.edu" {
		t.Errorf("expected email 'fallback@stanford.edu', got %q", capturedClaims.Email)
	}
}

func TestSessionMiddleware_SuperTokensError_EmailVerification(t *testing.T) {
	err := initSuperTokensForMiddlewareTest(
		func() (sessmodels.SessionContainer, error) {
			return newMockSessionContainer("st_user_network_err", map[string]interface{}{"email": "student@stanford.edu"}), nil
		},
		nil,
		func(userID, email string) (bool, error) {
			return false, errors.New("connection refused to supertokens core")
		},
		func(userID string) ([]string, error) {
			return []string{"member"}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to init supertokens: %v", err)
	}

	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/profile/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "AUTH_SERVICE_UNAVAILABLE" {
		t.Errorf("expected code AUTH_SERVICE_UNAVAILABLE, got %s", resp.Error.Code)
	}
}

func TestSessionMiddleware_SuperTokensError_UserRoles(t *testing.T) {
	err := initSuperTokensForMiddlewareTest(
		func() (sessmodels.SessionContainer, error) {
			return newMockSessionContainer("st_user_network_err", map[string]interface{}{"email": "student@stanford.edu"}), nil
		},
		nil,
		func(userID, email string) (bool, error) {
			return true, nil
		},
		func(userID string) ([]string, error) {
			return nil, errors.New("connection timeout to supertokens core")
		},
	)
	if err != nil {
		t.Fatalf("failed to init supertokens: %v", err)
	}

	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/profile/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "AUTH_SERVICE_UNAVAILABLE" {
		t.Errorf("expected code AUTH_SERVICE_UNAVAILABLE, got %s", resp.Error.Code)
	}
}

func TestSessionMiddleware_GetUserByIDError_Returns502(t *testing.T) {
	err := initSuperTokensForMiddlewareTest(
		func() (sessmodels.SessionContainer, error) {
			return newMockSessionContainer("st_user_no_email", map[string]interface{}{}), nil
		},
		func(userID string) (*epmodels.User, error) {
			return nil, errors.New("connection refused to supertokens core")
		},
		func(userID, email string) (bool, error) {
			return true, nil
		},
		func(userID string) ([]string, error) {
			return []string{"member"}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to init supertokens: %v", err)
	}

	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/profile/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "AUTH_SERVICE_UNAVAILABLE" {
		t.Errorf("expected code AUTH_SERVICE_UNAVAILABLE, got %s", resp.Error.Code)
	}
	expectedMsg := "Authentication identity provider unreachable"
	if resp.Error.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, resp.Error.Message)
	}
}

func TestSessionMiddleware_RejectsSessionWhenEmailCannotBeResolved(t *testing.T) {
	err := initSuperTokensForMiddlewareTest(
		func() (sessmodels.SessionContainer, error) {
			return newMockSessionContainer("st_user_no_email", map[string]interface{}{}), nil
		},
		func(userID string) (*epmodels.User, error) {
			return &epmodels.User{ID: userID, Email: ""}, nil
		},
		func(userID, email string) (bool, error) {
			return true, nil
		},
		func(userID string) ([]string, error) {
			return []string{"member"}, nil
		},
	)
	if err != nil {
		t.Fatalf("failed to init supertokens: %v", err)
	}

	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/profile/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
	expectedMsg := "User email could not be resolved from session"
	if resp.Error.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, resp.Error.Message)
	}
}

func TestSessionMiddleware_StaleUnverifiedToken_SynchronizesWhenFreshlyVerified(t *testing.T) {
	payload := map[string]interface{}{
		"email":         "stale@stanford.edu",
		"emailVerified": false,
		"roles":         []interface{}{"member"},
	}
	mergeCalled := false
	var mergedUpdate map[string]interface{}

	container := &sessmodels.TypeSessionContainer{
		GetUserID:              func() string { return "user_stale_verified" },
		GetUserIDWithContext:   func(userContext supertokens.UserContext) string { return "user_stale_verified" },
		GetTenantId:            func() string { return "public" },
		GetTenantIdWithContext: func(userContext supertokens.UserContext) string { return "public" },
		GetAccessTokenPayload: func() map[string]interface{} {
			return payload
		},
		GetAccessTokenPayloadWithContext: func(userContext supertokens.UserContext) map[string]interface{} {
			return payload
		},
		MergeIntoAccessTokenPayload: func(accessTokenPayloadUpdate map[string]interface{}) error {
			mergeCalled = true
			mergedUpdate = accessTokenPayloadUpdate
			for k, v := range accessTokenPayloadUpdate {
				payload[k] = v
			}
			return nil
		},
		AssertClaimsWithContext: func(claimValidators []claims.SessionClaimValidator, userContext supertokens.UserContext) error {
			return nil
		},
		AttachToRequestResponseWithContext: func(info sessmodels.RequestResponseInfo, userContext supertokens.UserContext) error {
			return nil
		},
	}

	emailVerificationCalled := false
	err := initSuperTokensForMiddlewareTest(
		func() (sessmodels.SessionContainer, error) {
			return container, nil
		},
		func(userID string) (*epmodels.User, error) {
			return &epmodels.User{
				ID:    userID,
				Email: "stale@stanford.edu",
			}, nil
		},
		func(userID, email string) (bool, error) {
			emailVerificationCalled = true
			if userID == "user_stale_verified" {
				return true, nil
			}
			return false, nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("failed to init supertokens: %v", err)
	}

	var capturedClaims *auth.UserClaims
	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserContext(r.Context())
		if err != nil {
			t.Fatalf("failed to get user context: %v", err)
		}
		capturedClaims = claims
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/profile/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
	if !emailVerificationCalled {
		t.Fatal("expected emailverification.IsEmailVerified to be queried when payload isVerified is false")
	}
	if capturedClaims == nil {
		t.Fatal("expected captured claims, got nil")
	}
	if !capturedClaims.EmailVerified {
		t.Fatalf("expected EmailVerified to be synchronized to true, got false")
	}
	if !mergeCalled {
		t.Fatal("expected MergeIntoAccessTokenPayload to be called")
	}
	if mergedUpdate["emailVerified"] != true {
		t.Fatalf("expected merged emailVerified to be true, got %v", mergedUpdate["emailVerified"])
	}
}

func TestSessionMiddleware_StaleUnverifiedToken_RemainsUnverifiedWhenNotFreshlyVerified(t *testing.T) {
	payload := map[string]interface{}{
		"email":         "stale@stanford.edu",
		"emailVerified": false,
		"roles":         []interface{}{"member"},
	}
	mergeCalled := false

	container := &sessmodels.TypeSessionContainer{
		GetUserID:              func() string { return "user_stale_unverified" },
		GetUserIDWithContext:   func(userContext supertokens.UserContext) string { return "user_stale_unverified" },
		GetTenantId:            func() string { return "public" },
		GetTenantIdWithContext: func(userContext supertokens.UserContext) string { return "public" },
		GetAccessTokenPayload: func() map[string]interface{} {
			return payload
		},
		GetAccessTokenPayloadWithContext: func(userContext supertokens.UserContext) map[string]interface{} {
			return payload
		},
		MergeIntoAccessTokenPayload: func(accessTokenPayloadUpdate map[string]interface{}) error {
			mergeCalled = true
			return nil
		},
		AssertClaimsWithContext: func(claimValidators []claims.SessionClaimValidator, userContext supertokens.UserContext) error {
			return nil
		},
		AttachToRequestResponseWithContext: func(info sessmodels.RequestResponseInfo, userContext supertokens.UserContext) error {
			return nil
		},
	}

	emailVerificationCalled := false
	err := initSuperTokensForMiddlewareTest(
		func() (sessmodels.SessionContainer, error) {
			return container, nil
		},
		func(userID string) (*epmodels.User, error) {
			return &epmodels.User{
				ID:    userID,
				Email: "stale@stanford.edu",
			}, nil
		},
		func(userID, email string) (bool, error) {
			emailVerificationCalled = true
			return false, nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("failed to init supertokens: %v", err)
	}

	var capturedClaims *auth.UserClaims
	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserContext(r.Context())
		if err != nil {
			t.Fatalf("failed to get user context: %v", err)
		}
		capturedClaims = claims
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/profile/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
	if !emailVerificationCalled {
		t.Fatal("expected emailverification.IsEmailVerified to be queried when payload isVerified is false")
	}
	if capturedClaims == nil {
		t.Fatal("expected captured claims, got nil")
	}
	if capturedClaims.EmailVerified {
		t.Fatalf("expected EmailVerified to remain false, got true")
	}
	if mergeCalled {
		t.Fatal("expected MergeIntoAccessTokenPayload NOT to be called when user is still unverified")
	}
}
