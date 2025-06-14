import tailwindcss from "@tailwindcss/vite";

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  modules: [
    "@nuxt/fonts",
    "@nuxt/icon",
    "@nuxt/test-utils",
    "@nuxt/image",
    "shadcn-nuxt",
    "@nuxtjs/color-mode",
    "nuxt-auth-utils",
  ],
  ssr: false,
  devtools: { enabled: true },
  css: ["~/assets/css/tailwind.css"],
  colorMode: {
    classSuffix: "",
  },
  runtimeConfig: {
    oauth: {
      keycloak: {
        clientId: process.env.NUXT_OAUTH_KEYCLOAK_CLIENT_ID,
        clientSecret: process.env.NUXT_OAUTH_KEYCLOAK_CLIENT_SECRET,
        serverUrl: process.env.NUXT_OAUTH_KEYCLOAK_SERVER_URL,
        realm: process.env.NUXT_OAUTH_KEYCLOAK_REALM,
        redirectURL: process.env.NUXT_OAUTH_KEYCLOAK_REDIRECT_URL,
      },
    },
    session: {
      password: process.env.NUXT_SESSION_PASSWORD || "",
    },
    proxy: {
      apiGatewayUrl: "http://localhost:8000",
    },
  },
  future: {
    compatibilityVersion: 4,
  },
  compatibilityDate: "2025-05-15",
  vite: {
    plugins: [
      tailwindcss(),
    ],
  },
  typescript: {
    typeCheck: true,
  },
  icon: {},
  shadcn: {
    prefix: "",
    componentDir: "./app/components/ui",
  },
});
