<script setup lang="ts">
import { useQuery } from "@tanstack/vue-query";
import { getCategories, getRequests } from "~/clients/core/sdk.gen";

definePageMeta({
  middleware: ["auth"],
});

const { data: requests, isLoading: requestLoading } = useQuery({
  queryKey: ["get-requests"],
  queryFn: () => getRequests({
    composable: "$fetch",
    path: { organizationId: 1 },
    query: { pageSize: 10 },
  }),
});

const { data: categories, isLoading: categoriesLoading } = useQuery({
  queryKey: ["get-categories"],
  queryFn: () => getCategories({
    composable: "$fetch",
    path: { organizationId: 1 },
  }),
});

const dataLoading = computed(() => categoriesLoading.value || requestLoading.value);
</script>

<template>
  <div class="container mx-auto flex flex-1 flex-col items-stretch">
    <h1 class="text-3xl font-semibold tracking-tight transition-colors first:mt-0">
      Requests
    </h1>
    <section class="w-full flex-1">
      <RequestsDataTable
        v-if="!dataLoading && requests && categories"
        :requests="requests?.requests"
        :categories="categories?.categories"
      />
    </section>
  </div>
</template>
