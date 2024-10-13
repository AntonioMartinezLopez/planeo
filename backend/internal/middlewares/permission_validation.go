package middlewares

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	cfg "planeo/api/config"
	"planeo/api/pkg/request"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

type Permission struct {
	Scopes       []string `json:"scopes"`
	ResourceId   string   `json:"rsid"`
	ResourceName string   `json:"rsname"`
}

func fetchUserPermissions(token string) (*[]Permission, error) {
	oauthIssuer := cfg.OauthIssuerUrl()
	requestURL := fmt.Sprintf("%s/protocol/openid-connect/token", oauthIssuer)

	data := map[string]string{
		"grant_type":    "urn:ietf:params:oauth:grant-type:uma-ticket",
		"response_mode": "permissions",
		"audience":      cfg.Config.KcOauthClientID,
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

func PermissionMiddleware(api huma.API, resourceName string, permission string) func(ctx huma.Context, next func(huma.Context)) {

	return func(ctx huma.Context, next func(huma.Context)) {
		accessToken, assertionCorrect := ctx.Context().Value(AccessTokenContextKey{}).(string)

		if !assertionCorrect {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		permissions, err := fetchUserPermissions(accessToken)

		if err != nil {
			huma.WriteErr(api, ctx, http.StatusInternalServerError, err.Error())
			return
		}

		for _, perm := range *permissions {
			if (resourceName == strings.ToLower(perm.ResourceName)) && (slices.Contains(perm.Scopes, permission)) {
				next(ctx)
				return
			}
		}

		huma.WriteErr(api, ctx, http.StatusUnauthorized, "no permission")
	}
}
