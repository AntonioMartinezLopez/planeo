import type { Category } from "~/clients/core/types.gen";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/vue-query";
import { getCategories, getRequests } from "~/clients/core/sdk.gen";

export default function (organizationId: number, initialPageSize: number = 10) {
  const pageSize = ref(initialPageSize);
  const cursor = ref(0);
  const nextCursor = ref<number | undefined>(0);
  const prevCursors = ref<number[]>([]);
  const selectedCategories = ref<number[]>([]);

  // Categories query
  const { data: categories } = useQuery({
    queryKey: ["get-requests", "get-categories"],
    queryFn: () => getCategories({
      composable: "$fetch",
      path: { organizationId: 1 },

    }),
  });

  // Watch for categories data changes with deep watching
  watch(
    () => categories.value?.categories,
    (newCategories) => {
      if (newCategories) {
        selectedCategories.value = newCategories.map((category: Category) => category.Id);
      }
    },
    { immediate: true, deep: true },
  );

  // Requests query
  const queryKey = computed(() => ["get-requests", cursor.value, pageSize.value, selectedCategories.value]);
  const queryEnabled = computed(() => selectedCategories.value.length > 0);

  const { data, isLoading } = useQuery({
    queryKey,
    queryFn: async () => await getRequests({
      composable: "$fetch",
      path: { organizationId },
      query: { pageSize: pageSize.value, cursor: cursor.value === 0 ? undefined : cursor.value },
    }),
    placeholderData: keepPreviousData,
    enabled: queryEnabled,
  });

  nextCursor.value = data.value?.nextCursor;

  watch(data, () => {
    nextCursor.value = data.value?.nextCursor;
  });

  watch(pageSize, () => {
    cursor.value = 0;
    prevCursors.value = [];
  });

  onBeforeUnmount(async () => {
    await useQueryClient().invalidateQueries({
      queryKey: ["get-requests"],
    });
  });

  // Pagination functions
  const isFirstPage = computed(() => prevCursors.value.length === 0);
  const isLastPage = computed(() => nextCursor.value! <= 1);

  function nextPage() {
    if (!isLastPage.value) {
      prevCursors.value.push(cursor.value);
      cursor.value = nextCursor.value!;
    }
  }

  function prevPage() {
    if (!isFirstPage.value) {
      cursor.value = prevCursors.value.pop()!;
    }
  }

  return {
    requests: computed(() => data.value?.requests ?? []),
    categories: computed(() => categories.value?.categories ?? []),
    selectedCategories,
    isLoading: computed(() => isLoading.value || !data.value),
    pageSize,
    cursor,
    nextCursor,
    nextPage,
    prevPage,
    isFirstPage,
    isLastPage,
  };
}
