export default defineNuxtRouteMiddleware(async () => {
  const { loggedIn, clear: clearSession, fetch: refreshSession } = useUserSession()

  await $fetch('/auth/refresh', { method: 'get' })
  await refreshSession()

  if (!loggedIn.value) {
    await clearSession()
    return await navigateTo('/login')
  }
})
