<template>
  <div>
    <p v-if="!loggedIn">
      You are not logged in.
      <a href="/auth/keycloak">Login with Keycloak</a>
    </p>
  </div>
</template>

<script setup lang="ts">
definePageMeta({
  layout: false,
})
const { loggedIn } = useUserSession()
</script>
