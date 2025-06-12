<script setup lang="ts">
definePageMeta({
  middleware: ["auth"],
});

// Note: this is just an example
// TODO: provide client library for accessing apis
async function fetchFromApi() {
  const { session } = useUserSession();
  const { error: fetchError }
  = useFetch(`/api/organizations/1/requests?pageSize=10`, { method: "get", headers:
  { authorization: `Bearer ${session.value?.tokens.access_token}`,
  } });

  if (fetchError.value) {
    console.error("Error fetching sample data", fetchError);
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
