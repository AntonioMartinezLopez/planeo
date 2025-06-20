<script setup lang="ts">
import { getRequests } from "~/clients/core/sdk.gen";

definePageMeta({
  middleware: ["auth"],
});

async function fetchFromApi() {
  const { error } = await getRequests({
    composable: "useFetch",
    path: { organizationId: 1 },
    query: { pageSize: 10 },
  });

  if (error.value) {
    console.error("Error fetching sample data", error);
  }
}
</script>

<template>
  <div class="container">
    <h1>Welcome to our Application</h1>
    <p>This is the home page of our application.</p>
    <Button @click="fetchFromApi">
      Fetch from API
    </Button>
  </div>
</template>
