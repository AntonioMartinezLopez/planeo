import type { UserSession } from "#auth-utils";

export default defineEventHandler(async (event) => {
  const apiGatewayUrl = useRuntimeConfig().proxy.apiGatewayUrl;
  const target = `${apiGatewayUrl}${event.path}`;

  const session: UserSession = await getUserSession(event);
  if (!session) {
    throw createError({
      status: 401,
      message: "No active session",
    });
  }
  if (!session.secure?.access_token) {
    throw createError({
      status: 401,
      message: "No access token",
    });
  }

  const { access_token } = session.secure;

  try {
    const req = await proxyRequest(event, target, {
      headers:
      {
        authorization: `Bearer ${access_token}`,
      },
    });
    return req;
  }
  catch (error) {
    console.error("API request failed:", error);
    throw createError({
      status: 500,
      message: "Internal Server Error",
    });
  }
});
