package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig is the static configuration the platform uses to talk to the
// upstream IdP (Authentik in production, anything OIDC-compliant otherwise).
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string // typically ["openid","profile","email"]
}

// OIDC encapsulates the OIDC client. Construct via NewOIDC at startup.
type OIDC struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

func NewOIDC(ctx context.Context, cfg OIDCConfig) (*OIDC, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return nil, fmt.Errorf("auth: OIDC config incomplete")
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: OIDC discovery: %w", err)
	}
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	return &OIDC{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
		},
	}, nil
}

// AuthCodeURL returns the URL the browser is redirected to in order to start
// the auth code flow. State is opaque CSRF protection; nonce binds the ID
// token to this exact authorization request. Both must be returned to the
// caller and stored (e.g. signed cookie) for the callback to compare.
func (o *OIDC) AuthCodeURL(state, nonce string) string {
	return o.oauth.AuthCodeURL(state, oidc.Nonce(nonce))
}

// Claims projects only what Portal needs from the verified ID token.
type Claims struct {
	Subject  string `json:"sub"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture"`
	Verified bool   `json:"email_verified"`
}

// Exchange completes the auth code flow:
//  1. Swaps the code for tokens.
//  2. Verifies the ID token signature, issuer, audience, expiry, and nonce.
//  3. Returns projected user claims.
//
// expectedNonce MUST be the same value previously passed to AuthCodeURL
// (typically retrieved from a signed/encrypted cookie set during /login).
func (o *OIDC) Exchange(ctx context.Context, code, expectedNonce string) (*Claims, error) {
	tok, err := o.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("auth: token exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("auth: missing id_token")
	}
	idTok, err := o.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("auth: id_token verify: %w", err)
	}
	if idTok.Nonce != expectedNonce {
		return nil, fmt.Errorf("auth: nonce mismatch")
	}
	var c Claims
	if err := idTok.Claims(&c); err != nil {
		return nil, fmt.Errorf("auth: claim decode: %w", err)
	}
	if c.Subject == "" {
		return nil, fmt.Errorf("auth: id_token missing sub")
	}
	return &c, nil
}

// RandomState generates a 32-byte URL-safe random string suitable for use as
// the OIDC `state` or `nonce` parameter.
func RandomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
