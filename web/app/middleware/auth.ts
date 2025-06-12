export default defineNuxtRouteMiddleware(async () => {
  const { loggedIn, clear: clearSession } = useUserSession();

  if (!loggedIn.value) {
    await clearSession();
    return await navigateTo("/login");
  }
});
