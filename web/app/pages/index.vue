<script setup lang="ts">
definePageMeta({
  middleware: ["auth"],
});

const {
  requests,
  categories,
  selectedCategories,
  isLoading,
  pageSize,
  isFirstPage,
  isLastPage,
  nextPage,
  prevPage,
} = usePaginatedRequests(1, 10);
</script>

<template>
  <div class="container mx-auto flex flex-1 flex-col items-stretch">
    <h1 class="text-3xl font-semibold tracking-tight transition-colors first:mt-0">
      Requests
    </h1>
    <section class="w-full flex-1">
      <RequestsDataTable
        v-if="!isLoading && requests && categories"
        v-model:page-size="pageSize"
        v-model:selected-categories="selectedCategories"
        :requests="requests || []"
        :categories="categories"
        :is-first-page="isFirstPage"
        :is-last-page="isLastPage"
        @next-page="nextPage"
        @prev-page="prevPage"
      />
    </section>
  </div>
</template>
