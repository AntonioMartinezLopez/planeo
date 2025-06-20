import type { CreateClientConfig } from "../core/client.gen";

export const createClientConfig: CreateClientConfig = (config) => {
  const basePath = window.location.origin;
  return {
    ...config,
    baseURL: `${basePath}/api`,
    retry: 1,
    retryStatusCodes: [401],
    retryDelay: 500, // can safely delete this
    onResponseError: async (context) => {
      if (context.response.status === 401) {
        try {
          await $fetch("/auth/refresh", { method: "GET" });
          await useUserSession().fetch();
        }
        catch {
          navigateTo("/login");
        }
      }
    },
  };
};
