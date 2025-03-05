package middlewares

import (
	"slices"
	"time"
)

type OauthAccessClaims struct {
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	UserId         string   `json:"userid"`
	Issuer         string   `json:"iss"`
	Groups         []string `json:"groups"`
	ExpirationTime int64    `json:"exp"`
	Roles          []string `json:"roles"`
}

func (c OauthAccessClaims) IsWithinOrganisation(organization string) bool {
	organizationWithPrefix := "/" + organization
	for _, group := range c.Groups {
		if group == organizationWithPrefix {
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
