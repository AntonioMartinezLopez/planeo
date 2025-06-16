import type { UserSession } from "#auth-utils";
import { isTokenExpired } from "~~/server/util/token-expired";

interface RefreshTokensResponse {
  access_token: string;
  refresh_token: string;
}

async function refreshTokens(refreshToken: string): Promise<RefreshTokensResponse> {
  const config = useRuntimeConfig();
  const keycloakUrl = config.oauth.keycloak.serverUrl;
  const realm = config.oauth.keycloak.realm;

  const result = await $fetch<RefreshTokensResponse>(`${keycloakUrl}/realms/${realm}/protocol/openid-connect/token`, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "refresh_token",
      client_id: config.oauth.keycloak.clientId,
      client_secret: config.oauth.keycloak.clientSecret,
      refresh_token: refreshToken,
    }).toString(),
  });

  return result;
}

export default defineEventHandler(async (event) => {
  const session: UserSession = await getUserSession(event);
  if (!session) {
    return;
  }
  if (!session.secure?.refresh_token) {
    return;
  }

  const { access_token, refresh_token } = session.secure;

  const isAccessTokenExpired = isTokenExpired(access_token);
  if (!isAccessTokenExpired) {
    return;
  }

  try {
    const newTokens = await refreshTokens(refresh_token);
    await setUserSession(event, {
      secure: {
        refresh_token: newTokens.refresh_token,
        access_token: newTokens.access_token,
      },
    });
  }
  catch (error) {
    console.error("Failed to refresh tokens:", error);
    await clearUserSession(event);
  }
});
