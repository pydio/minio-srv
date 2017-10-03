package auth

import (
	"context"
	"errors"

	"github.com/coreos/go-oidc"
	"github.com/micro/go-micro/metadata"
	"github.com/pydio/services/common"
	"golang.org/x/oauth2"
)

const (
	PYDIO_CONTEXT_CLAIMS_KEY = "claims"
)

type Claims struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Verified bool   `json:"email_verified"`
	Roles    string `json:"roles"`
}

func DefaultJWTVerifier() *JWTVerifier {
	return &JWTVerifier{
		IssuerUrl:    "http://127.0.0.1:5556/dex",
		ClientID:     "example-app",
		ClientSecret: "ZXhhbXBsZS1hcHAtc2VjcmV0",
	}
}

type JWTVerifier struct {
	IssuerUrl    string
	ClientID     string
	ClientSecret string
}

func (j *JWTVerifier) Verify(ctx context.Context, rawIDToken string) (context.Context, Claims, error) {

	claims := Claims{}
	provider, err := oidc.NewProvider(ctx, j.IssuerUrl)
	if err != nil {
		return ctx, claims, err
	}
	var verifier = provider.Verifier(&oidc.Config{ClientID: j.ClientID})

	// Parse and verify ID Token payload.
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return ctx, claims, err
	}

	// Extract custom claims

	if err := idToken.Claims(&claims); err != nil {
		return ctx, claims, err
	}

	if claims.Name == "" {
		return ctx, claims, errors.New("Cannot find name inside claims")
	}

	ctx = context.WithValue(ctx, PYDIO_CONTEXT_CLAIMS_KEY, claims)
	var md map[string]string
	var ok bool
	if md, ok = metadata.FromContext(ctx); !ok {
		md = make(map[string]string)
	}
	md[common.PYDIO_CONTEXT_USER_KEY] = claims.Name
	ctx = metadata.NewContext(ctx, md)

	return ctx, claims, nil

}

func (j *JWTVerifier) PasswordCredentialsToken(ctx context.Context, userName string, password string) (context.Context, Claims, error) {

	// Get JWT From Dex
	provider, _ := oidc.NewProvider(ctx, j.IssuerUrl)
	// Configure an OpenID Connect aware OAuth2 client.
	oauth2Config := oauth2.Config{
		ClientID:     j.ClientID,
		ClientSecret: j.ClientSecret,
		// Discovery returns the OAuth2 endpoints.
		Endpoint: provider.Endpoint(),
		// "openid" is a required scope for OpenID Connect flows.
		Scopes: []string{oidc.ScopeOpenID, "profile", "email", "pydio"},
	}

	claims := Claims{}

	if token, err := oauth2Config.PasswordCredentialsToken(ctx, userName, password); err == nil {

		idToken, _ := provider.Verifier(&oidc.Config{ClientID: j.ClientID}).Verify(ctx, token.Extra("id_token").(string))

		if e := idToken.Claims(&claims); e == nil {

			if claims.Name == "" {
				return ctx, claims, errors.New("No name inside Claims")
			}

			ctx = context.WithValue(ctx, PYDIO_CONTEXT_CLAIMS_KEY, claims)

			var md map[string]string
			var ok bool
			if md, ok = metadata.FromContext(ctx); !ok {
				md = make(map[string]string)
			}
			md[common.PYDIO_CONTEXT_USER_KEY] = claims.Name
			ctx = metadata.NewContext(ctx, md)

			return ctx, claims, nil

		} else {
			return ctx, claims, e
		}
	} else {
		return ctx, claims, err
	}

}
