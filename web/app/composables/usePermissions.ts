import { useQuery } from "@tanstack/vue-query";

async function fetchPermissions() {
  return $fetch("/auth/permissions", {
    retry: 1,
    retryStatusCodes: [401],
    retryDelay: 1000,
    onResponseError: async (context) => {
      if (context.response.status === 401) {
        try {
          await $fetch("/auth/refresh", { method: "GET" });
        }
        catch {
          await navigateTo("/login", { external: true });
        }
      }
    },
  });
}

export default async function () {
  const { data, error, refetch } = useQuery({ queryKey: ["get-permissions"], queryFn: fetchPermissions });

  return {
    data,
    error,
    refetch,
  };
}
