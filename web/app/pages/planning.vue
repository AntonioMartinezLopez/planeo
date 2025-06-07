<template>
  <div class="container">
    <h1>Planning site</h1>
    <p>This is a protected page of our application.</p>
    <button @click="logout">
      Logout
    </button>
  </div>
</template>

<script setup lang="ts">
definePageMeta({
  middleware: ['auth'],
  // or middleware: 'auth'
})

const { clear, fetch } = useUserSession()

const logout = async () => {
  await clear()
  await fetch()
  // navigateTo('/')
  // await fetch()
}
</script>
