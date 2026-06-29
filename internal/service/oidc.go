package service

import (
	"context"
	"fmt"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig holds OIDC provider settings loaded from env.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AdminClaim   string // claim name that carries role/group info
	AdminValue   string // value within that claim that means admin
}

// OIDCClaims is the subset of claims we extract from the ID token.
type OIDCClaims struct {
	Email     string
	Name      string
	AvatarURL string
	IsAdmin   bool
}

// OIDCService handles OIDC provider interaction.
type OIDCService struct {
	cfg      OIDCConfig
	provider *gooidc.Provider
	verifier *gooidc.IDTokenVerifier
	oauth2   oauth2.Config
}

// NewOIDCService initialises the OIDC provider. Returns nil, nil if Issuer is empty.
func NewOIDCService(ctx context.Context, cfg OIDCConfig) (*OIDCService, error) {
	if cfg.Issuer == "" {
		return nil, nil
	}
	provider, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	return &OIDCService{
		cfg:      cfg,
		provider: provider,
		verifier: provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

// AuthURL returns the provider redirect URL for the given state.
func (s *OIDCService) AuthURL(state string) string {
	return s.oauth2.AuthCodeURL(state)
}

// Exchange exchanges the auth code for claims.
func (s *OIDCService) Exchange(ctx context.Context, code string) (*OIDCClaims, error) {
	token, err := s.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange: %w", err)
	}

	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}

	idToken, err := s.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	c := &OIDCClaims{
		Email:     stringClaim(claims, "email"),
		Name:      stringClaim(claims, "name"),
		AvatarURL: stringClaim(claims, "picture"),
	}
	if s.cfg.AdminClaim != "" && s.cfg.AdminValue != "" {
		c.IsAdmin = claimContains(claims, s.cfg.AdminClaim, s.cfg.AdminValue)
	}
	return c, nil
}

func stringClaim(claims map[string]any, key string) string {
	v, _ := claims[key].(string)
	return v
}

// claimContains checks if a claim equals or contains the target value.
// Handles string and []any (array of strings) claim types.
func claimContains(claims map[string]any, key, target string) bool {
	v, ok := claims[key]
	if !ok {
		return false
	}
	switch val := v.(type) {
	case string:
		return val == target
	case []any:
		for _, item := range val {
			if s, ok := item.(string); ok && s == target {
				return true
			}
		}
	}
	return false
}
