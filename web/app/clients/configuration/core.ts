import type { CreateClientConfig } from "../core/client.gen";

export const createClientConfig: CreateClientConfig = (config) => {
  const basePath = window.location.origin;
  return {
    ...config,
    baseURL: `${basePath}/api`,
  };
};
