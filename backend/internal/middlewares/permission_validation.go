package middlewares

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
	oauthIssuer := os.Getenv("OAUTH_ISSUER")
	oauthClientID := os.Getenv("OAUTH_CLIENT_ID")
	requestURL := fmt.Sprintf("%s/protocol/openid-connect/token", oauthIssuer)

	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:uma-ticket")
	data.Set("response_mode", "permissions")
	data.Set("audience", oauthClientID)

	headers := map[string]string{
		"Content-Type":  "application/x-www-form-urlencoded",
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}

	response, err := request.HttpRequestWithRetry(request.POST, requestURL, data, headers)

	if err != nil {
		return nil, err
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
		}

		for _, perm := range *permissions {
			println(resourceName == strings.ToLower(perm.ResourceName))
			println(slices.Contains(perm.Scopes, permission))
			if (resourceName == strings.ToLower(perm.ResourceName)) && (slices.Contains(perm.Scopes, permission)) {
				next(ctx)
				return
			}
		}

		huma.WriteErr(api, ctx, http.StatusUnauthorized, "no permission")
	}
}
