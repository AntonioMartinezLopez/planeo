import type { UserSession } from "#auth-utils";

export default defineOAuthKeycloakEventHandler({
  config: {
    authorizationParams: {
      prompt: "login",
    },
  },
  async onSuccess(event, { user, tokens }) {
    const session: Omit<UserSession, "id"> = {
      user,
      secure: {
        refresh_token: tokens.refresh_token,
        access_token: tokens.access_token,
      },
    };
    await setUserSession(event, session);
    return sendRedirect(event, "/");
  },
  onError(event, error) {
    // Handle authentication errors
    console.error("Keycloak authentication error:", error);
    return sendRedirect(event, "/");
  },
});
