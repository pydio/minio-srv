package cmd

import (
	"fmt"
	"sync"
)

type authProviders struct {
	sync.RWMutex
	SAML samlProviders `json:"saml"`
	// Add new auth providers.
}

const minioIAM = "arn:minio:iam:"

func (a *authProviders) GetAllAuthProviders() map[string]struct{} {
	a.RLock()
	defer a.RUnlock()
	authProviderArns := make(map[string]struct{})
	for k, v := range a.SAML {
		if v.Enable {
			// Construct the auth ARN.
			authARN := minioIAM + serverConfig.GetRegion() + ":" + k + ":saml"
			authProviderArns[authARN] = struct{}{}
		}
	}
	return authProviderArns
}

func (a *authProviders) GetSAML() samlProviders {
	a.RLock()
	defer a.RUnlock()
	return a.SAML.Clone()
}

func (a *authProviders) GetSAMLByID(accountID string) samlProvider {
	a.RLock()
	defer a.RUnlock()
	return a.SAML[accountID]
}

func (a *authProviders) SetSAMLByID(accountID string, s samlProvider) {
	a.Lock()
	defer a.Unlock()
	a.SAML[accountID] = s
}

type samlProviders map[string]samlProvider

func (a samlProviders) Clone() samlProviders {
	a2 := make(samlProviders, len(a))
	for k, v := range a {
		a2[k] = v
	}
	return a2
}

func (a samlProviders) Validate() error {
	for k, v := range a {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("SAML [%s] configuration invalid: %s", k, err)
		}
	}
	return nil
}

type samlProvider struct {
	Enable     bool   `json:"enable"`
	IDP        string `json:"idP"`
	ProviderID string `json:"providerId"`
}

func (s samlProvider) Validate() error {
	if s.IDP != "" && s.ProviderID != "" {
		return nil
	}
	return fmt.Errorf("Invalid saml provider configuration %#v", s)
}
