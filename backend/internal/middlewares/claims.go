package middlewares

import (
	"slices"
	"time"
)

type OauthAccessClaims struct {
	// Permissions    []string `json:"permissions"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	UserId string `json:"userid"`
	Issuer string `json:"iss"`
	// Audiences      *[]string `json:"aud,omitempty"`
	AuthorizedParty string   `json:"azp"`
	ExpirationTime  int64    `json:"exp"`
	Roles           []string `json:"roles"`
}

func (c OauthAccessClaims) IsCorrectAzp(organization string) bool {
	return c.AuthorizedParty == organization
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
