import type { UserSession } from "#auth-utils";

export default defineEventHandler(async (event) => {
  const apiGatewayUrl = useRuntimeConfig().proxy.apiGatewayUrl;
  const target = `${apiGatewayUrl}${event.path}`;

  const session: UserSession = await getUserSession(event);
  if (!session) {
    return;
  }
  if (!session.secure?.access_token) {
    return;
  }

  const { access_token } = session.secure;

  return proxyRequest(event, target, {
    headers:
      {
        authorization: `Bearer ${access_token}`,
      },
  });
});
