import { useQuery } from "@tanstack/vue-query";
import { getOrganizations } from "~/clients/core/sdk.gen";

/**
 * Composable to fetch organizations for the authenticated user
 * Reads the user's sub claim from the session and fetches their organizations
 */
export default function useOrganization() {
  const { user } = useUserSession();

  const {
    data: organizations,
    isLoading,
    isError,
    error,
  } = useQuery({
    queryKey: ["organizations", user?.value?.sub],
    queryFn: () =>
      getOrganizations({
        composable: "$fetch",
      }),
    enabled: !!user?.value?.sub,
    staleTime: 1000 * 60 * 5, // Cache for 5 minutes
  });

  // Helper to get the first organization (most common use case)
  const organization = computed(() => organizations.value?.[0] ?? null);

  // Helper to get organization ID (most common use case)
  const organizationId = computed(() => organization.value?.id ?? null);

  return {
    organizations,
    organization,
    organizationId,
    isLoading,
    isError,
    error,
  };
}
