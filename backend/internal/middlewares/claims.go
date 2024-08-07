package middlewares

import (
	"slices"
	"time"
)

type OauthAccessClaims struct {
	Permissions    []string `json:"permissions"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	UserId         string   `json:"userid"`
	Issuer         string   `json:"iss"`
	Audiences      []string `json:"aud"`
	ExpirationTime int64    `json:"exp"`
	Roles          []string `json:"roles"`
}

func (c OauthAccessClaims) HasScope(expectedScope string) bool {
	for i := range c.Permissions {
		if c.Permissions[i] == expectedScope {
			return true
		}
	}
	return false
}

func (c OauthAccessClaims) HasAudience(expectedAudience string) bool {
	for i := range c.Audiences {
		if c.Audiences[i] == expectedAudience {
			return true
		}
	}
	return false
}

func (c OauthAccessClaims) HasIssuer(expectedIssuer string) bool {
	return c.Issuer == expectedIssuer
}

func (c OauthAccessClaims) IsExpired() bool {
	currentTime := time.Now().Unix()
	return currentTime > c.ExpirationTime
}

func (c OauthAccessClaims) IsRole(role string) bool {
	return slices.Contains(c.Roles, role)
}

type AccessClaimsContextKey struct{}
type AccessTokenContextKey struct{}
