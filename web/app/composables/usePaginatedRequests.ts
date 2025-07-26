import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/vue-query";
import { getRequests } from "~/clients/core/sdk.gen";

export default function (organizationId: number, initialPageSize: number = 10) {
  const pageSize = ref(initialPageSize);
  const cursor = ref(0);
  const nextCursor = ref<number | undefined>(0);
  const prevCursors = ref<number[]>([]);

  const queryKey = computed(() => ["get-requests", cursor.value]);

  const { data, isLoading } = useQuery({
    queryKey,
    queryFn: async () => await getRequests({
      composable: "$fetch",
      path: { organizationId },
      query: { pageSize: pageSize.value, cursor: cursor.value === 0 ? undefined : cursor.value },
    }),
    placeholderData: keepPreviousData,
    // refetchOnWindowFocus: false,
    // refetchOnMount: true,
  });

  nextCursor.value = data.value?.nextCursor;

  watch(data, () => {
    nextCursor.value = data.value?.nextCursor;
  });

  onBeforeUnmount(async () => {
    await useQueryClient().invalidateQueries({
      queryKey: ["get-requests"],
    });
  });

  // Pagination functions
  const isFirstPage = computed(() => prevCursors.value.length === 0);
  const isLastPage = computed(() => nextCursor.value! <= 1);

  function goToNextPage() {
    if (!isLastPage.value) {
      prevCursors.value.push(cursor.value);
      cursor.value = nextCursor.value!;
    }
  }

  function goToPrevPage() {
    if (!isFirstPage.value) {
      cursor.value = prevCursors.value.pop()!;
    }
  }

  return {
    requests: computed(() => data.value?.requests ?? []),
    isLoading,
    pageSize,
    cursor,
    nextCursor,
    goToNextPage,
    goToPrevPage,
    isFirstPage,
    isLastPage,

  };
}
