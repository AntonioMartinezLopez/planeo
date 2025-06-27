<script setup lang="ts">
import { useQuery } from "@tanstack/vue-query";
import { getRequests } from "~/clients/core/sdk.gen";

definePageMeta({
  middleware: ["auth"],
});

const permissions = await usePermissions();

const { data, isLoading } = useQuery({ queryKey: ["get-requests"], queryFn: () => getRequests({
  composable: "$fetch",
  path: { organizationId: 1 },
  query: { pageSize: 10 },
}) });
</script>

<template>
  <div class="container">
    <h1>Welcome to our Application</h1>
    <p>This is the index  page of our application.</p>
    <section>
      <h2>Permissions</h2>
      {{ permissions }}
    </section>
    <section>
      <h2>Data</h2>
      {{ isLoading ? 'is loading...' : data }}
    </section>
  </div>
</template>
