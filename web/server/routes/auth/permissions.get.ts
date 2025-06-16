import type { UserSession } from "#auth-utils";
import { isTokenExpired } from "~~/server/util/token-expired";

interface Permission {
  Scopes: string[];
  ResourceId: string;
  ResourceName: string;
}

async function getPermission(accessToken: string): Promise<Permission> {
  const config = useRuntimeConfig();
  const keycloakUrl = config.oauth.keycloak.serverUrl;
  const realm = config.oauth.keycloak.realm;

  const result = await $fetch<Permission>(`${keycloakUrl}/realms/${realm}/protocol/openid-connect/token`, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded", "authorization": `Bearer ${accessToken}` },
    body: new URLSearchParams({
      grant_type: "urn:ietf:params:oauth:grant-type:uma-ticket",
      response_mode: "permissions",
      audience: config.oauth.keycloak.clientId,
    }).toString(),
  });

  return result;
}

export default defineEventHandler(async (event) => {
  const session: UserSession = await getUserSession(event);
  if (!session) {
    return;
  }

  const access_token = session.secure?.access_token
  if (!access_token || isTokenExpired(access_token)) {
    return;
  }

  const permissions = await getPermission(access_token);

  return permissions;
});
