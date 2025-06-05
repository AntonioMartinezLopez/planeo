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
  ],
  ssr: false,
  devtools: { enabled: true },
  css: ['~/assets/css/tailwind.css'],
  colorMode: {
    classSuffix: '',
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
