package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const oidcFlowLifetime = 5 * time.Minute

// ClientSecretSourceは組織ごとのOIDC Client Secretを実行時に返す。
// Secretは永続化せず、環境変数やSecret Managerから供給する。
type ClientSecretSource func(organizationID string) (string, error)

// OIDCFlowは認可開始からコールバックまでサーバー側で保持する一時情報である。
type OIDCFlow struct {
	OrganizationID string
	State          string
	Nonce          string
	CodeVerifier   string
	ExpiresAt      time.Time
}

// OIDCAuthenticatorはAuthorization Code FlowとPKCEを実行する。
type OIDCAuthenticator struct {
	service      *Service
	httpClient   *http.Client
	clientSecret ClientSecretSource
	now          func() time.Time
}

func NewOIDCAuthenticator(service *Service, client *http.Client, clientSecret ClientSecretSource) (*OIDCAuthenticator, error) {
	if service == nil || client == nil || clientSecret == nil {
		return nil, ErrInvalid
	}
	return &OIDCAuthenticator{service: service, httpClient: client, clientSecret: clientSecret, now: time.Now}, nil
}

// BeginはIdPを検出し、state、nonce、PKCE付きの認可URLを生成する。
func (authenticator *OIDCAuthenticator) Begin(ctx context.Context, organizationID string) (string, OIDCFlow, error) {
	config, err := authenticator.service.OIDCConfiguration(organizationID)
	if err != nil {
		return "", OIDCFlow{}, ErrAuthentication
	}
	state, err := randomURLToken()
	if err != nil {
		return "", OIDCFlow{}, ErrAuthentication
	}
	nonce, err := randomURLToken()
	if err != nil {
		return "", OIDCFlow{}, ErrAuthentication
	}
	verifier, err := randomURLToken()
	if err != nil {
		return "", OIDCFlow{}, ErrAuthentication
	}

	provider, err := authenticator.provider(ctx, config.Issuer)
	if err != nil {
		return "", OIDCFlow{}, ErrAuthentication
	}
	oauthConfig := oauth2.Config{
		ClientID:    config.ClientID,
		Endpoint:    provider.Endpoint(),
		RedirectURL: config.RedirectURL,
		Scopes:      []string{oidc.ScopeOpenID, "profile", "email"},
	}
	flow := OIDCFlow{
		OrganizationID: organizationID,
		State:          state,
		Nonce:          nonce,
		CodeVerifier:   verifier,
		ExpiresAt:      authenticator.now().UTC().Add(oidcFlowLifetime),
	}
	authorizationURL := oauthConfig.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.S256ChallengeOption(verifier),
	)
	return authorizationURL, flow, nil
}

// Completeはstate、PKCE、ID Tokenの署名・issuer・audience・nonceを検証する。
func (authenticator *OIDCAuthenticator) Complete(ctx context.Context, flow OIDCFlow, returnedState, code string) (Principal, error) {
	failed := func() (Principal, error) {
		if validID(flow.OrganizationID) {
			_ = authenticator.service.RecordAuthentication(flow.OrganizationID, "", false)
		}
		return Principal{}, ErrAuthentication
	}
	if !validOIDCFlow(flow) || code == "" || authenticator.now().UTC().After(flow.ExpiresAt) || !constantTimeEqual(flow.State, returnedState) {
		return failed()
	}
	config, err := authenticator.service.OIDCConfiguration(flow.OrganizationID)
	if err != nil {
		return failed()
	}
	secret, err := authenticator.clientSecret(flow.OrganizationID)
	if err != nil || secret == "" {
		return failed()
	}
	provider, err := authenticator.provider(ctx, config.Issuer)
	if err != nil {
		return failed()
	}
	oauthConfig := oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: secret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  config.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	requestContext, cancel := context.WithTimeout(oidc.ClientContext(ctx, authenticator.httpClient), 10*time.Second)
	defer cancel()
	token, err := oauthConfig.Exchange(requestContext, code, oauth2.VerifierOption(flow.CodeVerifier))
	if err != nil {
		return failed()
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return failed()
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: config.ClientID}).Verify(requestContext, rawIDToken)
	if err != nil || idToken.Subject == "" || !constantTimeEqual(flow.Nonce, idToken.Nonce) {
		return failed()
	}
	principal, err := authenticator.service.PrincipalForSubject(flow.OrganizationID, idToken.Subject)
	if err != nil {
		return failed()
	}
	if err := authenticator.service.RecordAuthentication(principal.OrganizationID, principal.UserID, true); err != nil {
		return Principal{}, ErrAuthentication
	}
	return principal, nil
}

func (authenticator *OIDCAuthenticator) provider(ctx context.Context, issuer string) (*oidc.Provider, error) {
	requestContext, cancel := context.WithTimeout(oidc.ClientContext(ctx, authenticator.httpClient), 10*time.Second)
	defer cancel()
	return oidc.NewProvider(requestContext, issuer)
}

func randomURLToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func constantTimeEqual(expected, actual string) bool {
	if len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func validOIDCFlow(flow OIDCFlow) bool {
	if !validID(flow.OrganizationID) || flow.State == "" || flow.Nonce == "" || flow.CodeVerifier == "" || flow.ExpiresAt.IsZero() {
		return false
	}
	return validGeneratedToken(flow.State) && validGeneratedToken(flow.Nonce) && validGeneratedToken(flow.CodeVerifier)
}

func validGeneratedToken(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
