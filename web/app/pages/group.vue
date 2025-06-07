<template>
  <div class="container">
    <h1>Group site</h1>
    <p>This is a protected page of our application.</p>
  </div>
</template>

<script setup lang="ts">
definePageMeta({
  middleware: ['auth'],
})
</script>
