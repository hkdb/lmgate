package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/hkdb/lmgate/internal/models"
	"golang.org/x/oauth2"
)

type OIDCProvider struct {
	ID           string
	ProviderType string
	Verifier     *oidc.IDTokenVerifier
	OAuth2Config oauth2.Config
}

func NewOIDCProvider(providerType, clientID, clientSecret, issuerURL, redirectURL string) (*OIDCProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("creating OIDC provider: %w", err)
	}

	cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	return &OIDCProvider{
		ProviderType: providerType,
		Verifier:     verifier,
		OAuth2Config: cfg,
	}, nil
}

func (p *OIDCProvider) AuthURL(state, nonce string) string {
	return p.OAuth2Config.AuthCodeURL(state, oidc.Nonce(nonce))
}

type OIDCUserInfo struct {
	Sub         string `json:"sub"`
	Email       string `json:"email"`
	DisplayName string `json:"name"`
}

func (p *OIDCProvider) Exchange(ctx context.Context, code, nonce string) (*OIDCUserInfo, error) {
	token, err := p.OAuth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}

	idToken, err := p.Verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verifying id_token: %w", err)
	}

	if idToken.Nonce != nonce {
		return nil, fmt.Errorf("nonce mismatch")
	}

	var info OIDCUserInfo
	if err := idToken.Claims(&info); err != nil {
		return nil, fmt.Errorf("parsing claims: %w", err)
	}

	return &info, nil
}

func FindOrCreateOIDCUser(db *sql.DB, providerType string, info *OIDCUserInfo, adminEmail string) (*models.User, error) {
	user, err := models.GetUserByProviderSub(db, providerType, info.Sub)
	if err == nil {
		return user, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// Check if user exists by email (may have been created as local)
	user, err = models.GetUserByEmail(db, info.Email)
	if err == nil {
		// Link OIDC to existing user — we don't update auth_provider to keep existing login working
		return user, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	role := "user"
	if adminEmail != "" && info.Email == adminEmail {
		role = "admin"
	}

	return models.CreateUser(db, info.Email, info.DisplayName, "", role, providerType, info.Sub, false)
}
