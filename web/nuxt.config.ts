import tailwindcss from '@tailwindcss/vite'

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  modules: [
    '@nuxt/eslint',
    '@nuxt/fonts',
    '@nuxt/icon',
    '@nuxt/test-utils',
    '@nuxt/image',
    'shadcn-nuxt',
    '@nuxtjs/color-mode',
    'nuxt-auth-utils',
  ],
  ssr: false,
  devtools: { enabled: true },
  css: ['~/assets/css/tailwind.css'],
  colorMode: {
    classSuffix: '',
  },
  runtimeConfig: {
    oauth: {
      keycloak: {
        clientId: process.env.KEYCLOAK_CLIENT_ID || 'local',
        clientSecret: process.env.KEYCLOAK_CLIENT_SECRET || 't4VlYX9CJIN3VTrlb5nRMXT8Qjr9SBdu',
        serverUrl: process.env.KEYCLOAK_SERVER_URL || 'http://localhost:8080',
        realm: process.env.KEYCLOAK_REALM || 'local',
        redirectURL: process.env.KEYCLOAK_REDIRECT_URL || 'http://localhost:3000/auth/keycloak',
      },
    },
    session: {
      password: process.env.NUXT_SESSION_PASSWORD || '',
    },
  },
  future: {
    compatibilityVersion: 4,
  },
  compatibilityDate: '2025-05-15',
  vite: {
    plugins: [
      tailwindcss(),
    ],
  },
  typescript: {
    typeCheck: true,
  },
  eslint: {
    config: {
      stylistic: true,
    },
  },
  icon: {},
  shadcn: {
    prefix: '',
    componentDir: './app/components/ui',
  },
})
