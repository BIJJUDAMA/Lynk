package auth

import (
	"errors"
	"net/mail"
	"strings"

	"github.com/supertokens/supertokens-golang/recipe/emailpassword"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword/epmodels"
	"github.com/supertokens/supertokens-golang/recipe/emailverification"
	"github.com/supertokens/supertokens-golang/recipe/emailverification/evmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"github.com/supertokens/supertokens-golang/supertokens"
)

// SuperTokensConfig encapsulates connection and domain settings for SuperTokens.
type SuperTokensConfig struct {
	ConnectionURI string
	APIKey        string
	APIDomain     string
	WebsiteDomain string
	APIBasePath   string
}

// IsEduEmail strictly validates that an email address has a valid institutional .edu domain.
func IsEduEmail(email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return false
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address == "" {
		return false
	}

	// Double check that full email matches the parsed address (no display names like "John Doe <john@mit.edu>")
	if addr.Address != email {
		return false
	}

	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}

	domain := parts[1]
	// Must end with .edu
	if !strings.HasSuffix(domain, ".edu") {
		return false
	}

	domainParts := strings.Split(domain, ".")
	if len(domainParts) < 2 {
		return false
	}

	// Ensure all domain labels are non-empty
	for _, label := range domainParts {
		if label == "" {
			return false
		}
	}

	// Ensure top-level domain is strictly "edu"
	return domainParts[len(domainParts)-1] == "edu"
}

// InitSupertokens initializes the SuperTokens Go SDK with emailpassword (.edu validation),
// required email verification, user sessions, and roles.
func InitSupertokens(cfg SuperTokensConfig) error {
	apiBasePath := cfg.APIBasePath
	if apiBasePath == "" {
		apiBasePath = "/api/v1/auth"
	}
	websiteBasePath := "/auth"

	return supertokens.Init(supertokens.TypeInput{
		Supertokens: &supertokens.ConnectionInfo{
			ConnectionURI: cfg.ConnectionURI,
			APIKey:        cfg.APIKey,
		},
		AppInfo: supertokens.AppInfo{
			AppName:         "Lynk",
			APIDomain:       cfg.APIDomain,
			WebsiteDomain:   cfg.WebsiteDomain,
			APIBasePath:     &apiBasePath,
			WebsiteBasePath: &websiteBasePath,
		},
		RecipeList: []supertokens.Recipe{
			emailpassword.Init(&epmodels.TypeInput{
				Override: &epmodels.OverrideStruct{
					Functions: func(originalImplementation epmodels.RecipeInterface) epmodels.RecipeInterface {
						ogSignUp := *originalImplementation.SignUp
						(*originalImplementation.SignUp) = func(email, password string, tenantId string, userContext supertokens.UserContext) (epmodels.SignUpResponse, error) {
							if !IsEduEmail(email) {
								return epmodels.SignUpResponse{}, errors.New("registration rejected: only institutional .edu email addresses are permitted")
							}
							return ogSignUp(email, password, tenantId, userContext)
						}
						return originalImplementation
					},
					APIs: func(originalImplementation epmodels.APIInterface) epmodels.APIInterface {
						ogSignUpPOST := *originalImplementation.SignUpPOST
						(*originalImplementation.SignUpPOST) = func(formFields []epmodels.TypeFormField, tenantId string, options epmodels.APIOptions, userContext supertokens.UserContext) (epmodels.SignUpPOSTResponse, error) {
							var email string
							for _, field := range formFields {
								if field.ID == "email" {
									if s, ok := field.Value.(string); ok {
										email = s
									}
								}
							}
							if !IsEduEmail(email) {
								return epmodels.SignUpPOSTResponse{
									GeneralError: &supertokens.GeneralErrorResponse{
										Message: "Registration rejected: only institutional .edu email addresses are permitted.",
									},
								}, nil
							}
							return ogSignUpPOST(formFields, tenantId, options, userContext)
						}
						return originalImplementation
					},
				},
			}),
			emailverification.Init(evmodels.TypeInput{
				Mode: evmodels.ModeOptional,
				Override: &evmodels.OverrideStruct{
					APIs: func(originalImplementation evmodels.APIInterface) evmodels.APIInterface {
						ogVerifyEmailPOST := *originalImplementation.VerifyEmailPOST
						(*originalImplementation.VerifyEmailPOST) = func(token string, sessionContainer sessmodels.SessionContainer, tenantId string, options evmodels.APIOptions, userContext supertokens.UserContext) (evmodels.VerifyEmailPOSTResponse, error) {
							resp, err := ogVerifyEmailPOST(token, sessionContainer, tenantId, options, userContext)
							if err == nil && sessionContainer != nil && sessionContainer.MergeIntoAccessTokenPayload != nil {
								_ = sessionContainer.MergeIntoAccessTokenPayload(map[string]interface{}{
									"emailVerified": true,
								})
							}
							return resp, err
						}
						return originalImplementation
					},
				},
			}),
			session.Init(&sessmodels.TypeInput{
				ExposeAccessTokenToFrontendInCookieBasedAuth: true,
				Override: &sessmodels.OverrideStruct{
					Functions: func(originalImplementation sessmodels.RecipeInterface) sessmodels.RecipeInterface {
						ogCreateNewSession := *originalImplementation.CreateNewSession
						(*originalImplementation.CreateNewSession) = func(userID string, accessTokenPayload map[string]interface{}, sessionDataInDatabase map[string]interface{}, disableAntiCsrf *bool, tenantId string, userContext supertokens.UserContext) (sessmodels.SessionContainer, error) {
							if accessTokenPayload == nil {
								accessTokenPayload = map[string]interface{}{}
							}
							// Fetch email from SuperTokens user store if missing
							emailVal, ok := accessTokenPayload["email"].(string)
							if !ok || strings.TrimSpace(emailVal) == "" {
								if user, err := emailpassword.GetUserByID(userID, userContext); err == nil && user != nil {
									accessTokenPayload["email"] = user.Email
								}
							}
							if _, exists := accessTokenPayload["emailVerified"]; !exists {
								verified, vErr := emailverification.IsEmailVerified(userID, nil)
								if vErr != nil {
									verified = false
								}
								accessTokenPayload["emailVerified"] = verified
							}
							if _, exists := accessTokenPayload["roles"]; !exists {
								rolesRes, rErr := userroles.GetRolesForUser("public", userID)
								roles := []string{"member"}
								if rErr == nil && rolesRes.OK != nil && len(rolesRes.OK.Roles) > 0 {
									roles = rolesRes.OK.Roles
								}
								accessTokenPayload["roles"] = roles
							}
							return ogCreateNewSession(userID, accessTokenPayload, sessionDataInDatabase, disableAntiCsrf, tenantId, userContext)
						}
						return originalImplementation
					},
				},
			}),
			userroles.Init(nil),
		},
	})
}
