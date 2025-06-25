package middlewares

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"planeo/libs/request"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

type Permission struct {
	Scopes       []string `json:"scopes"`
	ResourceId   string   `json:"rsid"`
	ResourceName string   `json:"rsname"`
}

func fetchUserPermissions(issuer string, audience string, token string) (*[]Permission, error) {
	requestURL := fmt.Sprintf("%s/protocol/openid-connect/token", issuer)

	data := map[string]string{
		"grant_type":    "urn:ietf:params:oauth:grant-type:uma-ticket",
		"response_mode": "permissions",
		"audience":      audience,
	}

	headers := map[string]string{
		"Content-Type":  "application/x-www-form-urlencoded",
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}

	requestParams := request.HttpRequestParams{
		Method:      request.POST,
		URL:         requestURL,
		Headers:     headers,
		Body:        data,
		ContentType: request.ApplicationFormURLEncoded,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("permissions fetching was not successful: %d", response.StatusCode)
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	var permissions []Permission
	err = json.Unmarshal(body, &permissions)

	if err != nil {
		return nil, err
	}

	return &permissions, nil
}

type PermissionMiddlewareConfig struct {
	api       huma.API
	IssuerUrl string
	ClientId  string
}

func NewPermissionMiddlewareConfig(api huma.API, issuerUrl string, clientId string) *PermissionMiddlewareConfig {
	return &PermissionMiddlewareConfig{
		api:       api,
		IssuerUrl: issuerUrl,
		ClientId:  clientId,
	}
}

func (p *PermissionMiddlewareConfig) Apply(resourceName string, permission string) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		accessToken, assertionCorrect := ctx.Context().Value(AccessTokenContextKey{}).(string)

		if !assertionCorrect {
			huma.WriteErr(p.api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		permissions, err := fetchUserPermissions(p.IssuerUrl, p.ClientId, accessToken)

		if err != nil {
			huma.WriteErr(p.api, ctx, http.StatusUnauthorized, err.Error())
			return
		}

		for _, perm := range *permissions {
			if (resourceName == strings.ToLower(perm.ResourceName)) && (slices.Contains(perm.Scopes, permission)) {
				next(ctx)
				return
			}
		}

		huma.WriteErr(p.api, ctx, http.StatusUnauthorized, "no permission")
	}
}
