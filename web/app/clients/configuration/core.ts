import type { CreateClientConfig } from "../core/client.gen";

export const createClientConfig: CreateClientConfig = (config) => {
  const basePath = window.location.origin;
  return {
    ...config,
    baseURL: `${basePath}/api`,
    retry: 1,
    retryStatusCodes: [401],
    retryDelay: 1000,
    onResponseError: async (context) => {
      if (context.response.status === 401) {
        try {
          await $fetch("/auth/refresh", { method: "GET" });
        }
        catch {
          console.error("Failed to refresh tokens, redirecting to login");
          await navigateTo("/", { external: true });
        }
      }
    },
  };
};
