package auth_test

import (
	"strings"
	"testing"

	"github.com/lynk/backend/internal/auth"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword/epmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/supertokens"
)

func TestIsEduEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		// Valid institutional .edu addresses
		{"standard university address", "student@stanford.edu", true},
		{"subdomain university address", "jordan.lee@cs.berkeley.edu", true},
		{"faculty address", "faculty@mit.edu", true},
		{"uppercase university address", "ALUMNI@HARVARD.EDU", true},
		{"multi-level subdomain address", "user@mail.subdomain.univ.edu", true},

		// Invalid non-.edu addresses (MUST be rejected)
		{"commercial domain .com", "user@gmail.com", false},
		{"country-code UK domain", "student@yahoo.co.uk", false},
		{"prefix domain .edu.com", "hacker@edu.com", false},
		{"hyphenated fake edu domain", "attacker@fake-edu.org", false},
		{"subdomain containing edu ending in .com", "user@stanford.edu.evil.com", false},
		{"plain string without at sign", "plainaddress", false},
		{"missing local part", "@stanford.edu", false},
		{"missing domain part", "user@", false},
		{"empty string", "", false},
		{"consecutive dots before edu", "user@stanford..edu", false},
		{"empty subdomain dot edu", "user@.edu", false},
		{"naked edu domain", "user@edu", false},
		{"display name with edu email", "John Doe <student@mit.edu>", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := auth.IsEduEmail(tt.email)
			if got != tt.expected {
				t.Errorf("IsEduEmail(%q) = %v; expected %v", tt.email, got, tt.expected)
			}
		})
	}
}

func TestSuperTokensConfig_Struct(t *testing.T) {
	cfg := auth.SuperTokensConfig{
		ConnectionURI: "http://localhost:3567",
		APIKey:        "test-api-key",
		APIDomain:     "http://localhost:8080",
		WebsiteDomain: "http://localhost:3000",
		APIBasePath:   "/api/v1/auth",
	}

	if cfg.ConnectionURI != "http://localhost:3567" {
		t.Errorf("expected ConnectionURI 'http://localhost:3567', got %s", cfg.ConnectionURI)
	}
	if cfg.APIKey != "test-api-key" {
		t.Errorf("expected APIKey 'test-api-key', got %s", cfg.APIKey)
	}
	if cfg.APIDomain != "http://localhost:8080" {
		t.Errorf("expected APIDomain 'http://localhost:8080', got %s", cfg.APIDomain)
	}
	if cfg.WebsiteDomain != "http://localhost:3000" {
		t.Errorf("expected WebsiteDomain 'http://localhost:3000', got %s", cfg.WebsiteDomain)
	}
	if cfg.APIBasePath != "/api/v1/auth" {
		t.Errorf("expected APIBasePath '/api/v1/auth', got %s", cfg.APIBasePath)
	}
}

func TestInitSupertokens(t *testing.T) {
	supertokens.ResetForTest()

	cfg := auth.SuperTokensConfig{
		ConnectionURI: "http://localhost:3567",
		APIKey:        "some-api-key",
		APIDomain:     "http://localhost:8080",
		WebsiteDomain: "http://localhost:3000",
		APIBasePath:   "/api/v1/auth",
	}

	err := auth.InitSupertokens(cfg)
	if err != nil {
		t.Fatalf("InitSupertokens returned unexpected error: %v", err)
	}
}

func TestSignUpOverride_RejectsNonEdu(t *testing.T) {
	supertokens.ResetForTest()

	cfg := auth.SuperTokensConfig{
		ConnectionURI: "http://localhost:3567",
		APIKey:        "some-api-key",
		APIDomain:     "http://localhost:8080",
		WebsiteDomain: "http://localhost:3000",
	}

	err := auth.InitSupertokens(cfg)
	if err != nil {
		t.Fatalf("InitSupertokens failed: %v", err)
	}

	// 1. Verify Function SignUp rejection for non-.edu
	_, err = emailpassword.SignUp("public", "attacker@gmail.com", "validPass123!")
	if err == nil {
		t.Fatal("expected emailpassword.SignUp to reject non-.edu email, got nil error")
	}
	if !strings.Contains(err.Error(), "only institutional .edu email addresses are permitted") {
		t.Errorf("expected error message containing '.edu', got: %v", err)
	}

	// 2. Verify API SignUpPOST rejection for non-.edu returning GeneralErrorResponse
	recipe, err := emailpassword.GetRecipeInstanceOrThrowError()
	if err != nil {
		t.Fatalf("failed to get recipe instance: %v", err)
	}

	formFields := []epmodels.TypeFormField{
		{ID: "email", Value: "attacker@gmail.com"},
		{ID: "password", Value: "validPass123!"},
	}
	userCtx := &map[string]interface{}{}

	postResp, err := (*recipe.APIImpl.SignUpPOST)(formFields, "public", epmodels.APIOptions{}, userCtx)
	if err != nil {
		t.Fatalf("SignUpPOST returned unexpected error: %v", err)
	}
	if postResp.GeneralError == nil {
		t.Fatal("expected GeneralError in SignUpPOSTResponse for non-.edu email, got nil")
	}
	expectedMsg := "Registration rejected: only institutional .edu email addresses are permitted."
	if postResp.GeneralError.Message != expectedMsg {
		t.Errorf("expected GeneralError message %q, got %q", expectedMsg, postResp.GeneralError.Message)
	}
}

func TestSessionInit_InjectsEmailIntoAccessTokenPayload(t *testing.T) {
	supertokens.ResetForTest()

	cfg := auth.SuperTokensConfig{
		ConnectionURI: "http://localhost:3567",
		APIKey:        "some-api-key",
		APIDomain:     "http://localhost:8080",
		WebsiteDomain: "http://localhost:3000",
	}

	err := auth.InitSupertokens(cfg)
	if err != nil {
		t.Fatalf("InitSupertokens failed: %v", err)
	}

	sessRecipe, err := session.GetRecipeInstanceOrThrowError()
	if err != nil {
		t.Fatalf("failed to get session recipe instance: %v", err)
	}

	epRecipe, err := emailpassword.GetRecipeInstanceOrThrowError()
	if err != nil {
		t.Fatalf("failed to get emailpassword recipe instance: %v", err)
	}
	mockUser := &epmodels.User{
		ID:    "user_123",
		Email: "student@stanford.edu",
	}
	getUserByIDFn := func(userID string, userContext supertokens.UserContext) (*epmodels.User, error) {
		if userID == "user_123" {
			return mockUser, nil
		}
		return nil, nil
	}
	epRecipe.RecipeImpl.GetUserByID = &getUserByIDFn

	payload := map[string]interface{}{}
	userCtx := &map[string]interface{}{}

	// Call CreateNewSession on the recipe implementation
	_, _ = (*sessRecipe.RecipeImpl.CreateNewSession)("user_123", payload, nil, nil, "public", userCtx)

	if payload["email"] != "student@stanford.edu" {
		t.Errorf("expected email 'student@stanford.edu' in payload, got %v", payload["email"])
	}
}

func TestSessionInit_InjectsEmailIntoAccessTokenPayload_WhenEmptyString(t *testing.T) {
	supertokens.ResetForTest()

	cfg := auth.SuperTokensConfig{
		ConnectionURI: "http://localhost:3567",
		APIKey:        "some-api-key",
		APIDomain:     "http://localhost:8080",
		WebsiteDomain: "http://localhost:3000",
	}

	err := auth.InitSupertokens(cfg)
	if err != nil {
		t.Fatalf("InitSupertokens failed: %v", err)
	}

	sessRecipe, err := session.GetRecipeInstanceOrThrowError()
	if err != nil {
		t.Fatalf("failed to get session recipe instance: %v", err)
	}

	epRecipe, err := emailpassword.GetRecipeInstanceOrThrowError()
	if err != nil {
		t.Fatalf("failed to get emailpassword recipe instance: %v", err)
	}
	mockUser := &epmodels.User{
		ID:    "user_123",
		Email: "student@stanford.edu",
	}
	getUserByIDFn := func(userID string, userContext supertokens.UserContext) (*epmodels.User, error) {
		if userID == "user_123" {
			return mockUser, nil
		}
		return nil, nil
	}
	epRecipe.RecipeImpl.GetUserByID = &getUserByIDFn

	payload := map[string]interface{}{"email": "   "}
	userCtx := &map[string]interface{}{}

	_, _ = (*sessRecipe.RecipeImpl.CreateNewSession)("user_123", payload, nil, nil, "public", userCtx)

	if payload["email"] != "student@stanford.edu" {
		t.Errorf("expected email 'student@stanford.edu' in payload, got %v", payload["email"])
	}
}

func TestSessionInit_InjectsEmailVerifiedAndRolesIntoAccessTokenPayload(t *testing.T) {
	supertokens.ResetForTest()

	cfg := auth.SuperTokensConfig{
		ConnectionURI: "http://localhost:3567",
		APIKey:        "some-api-key",
		APIDomain:     "http://localhost:8080",
		WebsiteDomain: "http://localhost:3000",
	}

	err := auth.InitSupertokens(cfg)
	if err != nil {
		t.Fatalf("InitSupertokens failed: %v", err)
	}

	sessRecipe, err := session.GetRecipeInstanceOrThrowError()
	if err != nil {
		t.Fatalf("failed to get session recipe instance: %v", err)
	}

	epRecipe, err := emailpassword.GetRecipeInstanceOrThrowError()
	if err != nil {
		t.Fatalf("failed to get emailpassword recipe instance: %v", err)
	}
	mockUser := &epmodels.User{
		ID:    "user_123",
		Email: "student@stanford.edu",
	}
	getUserByIDFn := func(userID string, userContext supertokens.UserContext) (*epmodels.User, error) {
		if userID == "user_123" {
			return mockUser, nil
		}
		return nil, nil
	}
	epRecipe.RecipeImpl.GetUserByID = &getUserByIDFn

	payload := map[string]interface{}{}
	userCtx := &map[string]interface{}{}

	_, _ = (*sessRecipe.RecipeImpl.CreateNewSession)("user_123", payload, nil, nil, "public", userCtx)

	captured := payload
	if _, ok := captured["email"].(string); !ok {
		t.Fatal("expected email in access token payload")
	}
	if _, ok := captured["emailVerified"].(bool); !ok {
		t.Fatal("expected emailVerified bool in access token payload")
	}
	roles, ok := captured["roles"].([]string)
	if !ok {
		if raw, ok2 := captured["roles"].([]interface{}); ok2 {
			roles = make([]string, len(raw))
			for i, v := range raw {
				roles[i], _ = v.(string)
			}
		} else {
			t.Fatalf("expected roles slice in payload, got %#v", captured["roles"])
		}
	}
	if len(roles) == 0 {
		t.Fatal("expected at least one role in access token payload")
	}
}

