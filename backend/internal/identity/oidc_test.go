package identity

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

func TestOIDCAuthorizationCodeFlowUsesPKCEAndAuthenticatesSubject(t *testing.T) {
	service := testService(t)
	admin := bootstrap(t, service)
	_, err := service.CreateUser(admin, CreateUserInput{ID: "reviewer", Subject: "idp-reviewer", Email: "reviewer@acme.test", DisplayName: "レビュー担当", Roles: []Role{RoleReviewer}})
	if err != nil {
		t.Fatal(err)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var issuer, expectedVerifier, tokenNonce string
	tokenRequested := false
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/.well-known/openid-configuration":
			writeTestJSON(response, map[string]any{
				"issuer":                                issuer,
				"authorization_endpoint":                issuer + "/authorize",
				"token_endpoint":                        issuer + "/token",
				"jwks_uri":                              issuer + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
				"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
			})
		case "/keys":
			writeTestJSON(response, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &privateKey.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig"}}})
		case "/token":
			tokenRequested = true
			if err := request.ParseForm(); err != nil {
				t.Errorf("token form: %v", err)
			}
			_, basicSecret, hasBasicAuthentication := request.BasicAuth()
			validSecret := request.Form.Get("client_secret") == "runtime-secret" || hasBasicAuthentication && basicSecret == "runtime-secret"
			if !validSecret || request.Form.Get("code_verifier") != expectedVerifier {
				t.Errorf("missing client authentication or PKCE verifier")
			}
			rawToken, err := signedTestIDToken(privateKey, issuer, tokenNonce)
			if err != nil {
				t.Errorf("sign token: %v", err)
				response.WriteHeader(http.StatusInternalServerError)
				return
			}
			writeTestJSON(response, map[string]any{"access_token": "opaque-access-token", "token_type": "Bearer", "expires_in": 300, "id_token": rawToken})
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	issuer = server.URL

	if _, err := service.ConfigureOIDC(admin, issuer, "reprokit-client", "http://127.0.0.1:4567/api/auth/oidc/callback"); err != nil {
		t.Fatal(err)
	}
	authenticator, err := NewOIDCAuthenticator(service, server.Client(), func(organizationID string) (string, error) {
		if organizationID != "acme" {
			return "", errors.New("unknown organization")
		}
		return "runtime-secret", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	authenticator.now = service.now

	authorizationURL, flow, err := authenticator.Begin(t.Context(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("state") != flow.State || parsed.Query().Get("nonce") != flow.Nonce || parsed.Query().Get("code_challenge_method") != "S256" || parsed.Query().Get("code_challenge") == "" {
		t.Fatalf("authorization URL lacks OIDC protections: %s", authorizationURL)
	}
	expectedVerifier = flow.CodeVerifier
	tokenNonce = flow.Nonce
	principal, err := authenticator.Complete(t.Context(), flow, flow.State, "authorization-code")
	if err != nil {
		t.Fatal(err)
	}
	if !tokenRequested || principal != (Principal{OrganizationID: "acme", UserID: "reviewer"}) {
		t.Fatalf("unexpected authentication result: %#v", principal)
	}
	if err := service.Authorize(principal, PermissionReviewSubmissions); err != nil {
		t.Fatalf("authenticated principal lost its role: %v", err)
	}

	events, err := service.ListAuditEvents(admin)
	if err != nil || !containsAuditAction(events, AuditAuthenticationOK) {
		t.Fatalf("successful authentication was not audited: %#v %v", events, err)
	}
}

func TestOIDCFlowRejectsTamperedStateAndExpiredFlow(t *testing.T) {
	service := testService(t)
	admin := bootstrap(t, service)
	if _, err := service.ConfigureOIDC(admin, "http://127.0.0.1:4321", "client", "http://127.0.0.1:4567/callback"); err != nil {
		t.Fatal(err)
	}
	authenticator, err := NewOIDCAuthenticator(service, http.DefaultClient, func(string) (string, error) { return "secret", nil })
	if err != nil {
		t.Fatal(err)
	}
	authenticator.now = service.now
	flow := OIDCFlow{
		OrganizationID: "acme",
		State:          testGeneratedToken(1),
		Nonce:          testGeneratedToken(2),
		CodeVerifier:   testGeneratedToken(3),
		ExpiresAt:      service.now().UTC().Add(time.Minute),
	}
	if _, err := authenticator.Complete(t.Context(), flow, testGeneratedToken(4), "code"); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("tampered state was accepted: %v", err)
	}
	flow.ExpiresAt = service.now().UTC().Add(-time.Second)
	if _, err := authenticator.Complete(t.Context(), flow, flow.State, "code"); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("expired flow was accepted: %v", err)
	}
	events, err := service.ListAuditEvents(admin)
	if err != nil {
		t.Fatal(err)
	}
	failures := 0
	for _, event := range events {
		if event.Action == AuditAuthenticationFailed {
			failures++
		}
	}
	if failures != 2 {
		t.Fatalf("authentication failures were not audited: %#v", events)
	}
}

func signedTestIDToken(privateKey *rsa.PrivateKey, issuer, nonce string) (string, error) {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"))
	if err != nil {
		return "", err
	}
	now := time.Now()
	return jwt.Signed(signer).Claims(&jwt.Claims{
		Issuer:   issuer,
		Subject:  "idp-reviewer",
		Audience: jwt.Audience{"reprokit-client"},
		IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)),
		Expiry:   jwt.NewNumericDate(now.Add(time.Hour)),
	}).Claims(struct {
		Nonce string `json:"nonce"`
	}{Nonce: nonce}).Serialize()
}

func writeTestJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(value)
}

func containsAuditAction(events []AuditEvent, action AuditAction) bool {
	for _, event := range events {
		if event.Action == action {
			return true
		}
	}
	return false
}

func testGeneratedToken(fill byte) string {
	value := make([]byte, 32)
	for index := range value {
		value[index] = fill
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
