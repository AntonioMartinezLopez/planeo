export default defineNuxtRouteMiddleware(async () => {
  const { loggedIn, fetch, clear: clearSession } = useUserSession();

  await fetch();

  if (!loggedIn.value) {
    await clearSession();
    return await navigateTo("/login");
  }
});
