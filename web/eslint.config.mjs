// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt(
  // Your custom configs here
  [
    {
      files: ['pages/**/*.{vue,js,ts}', 'layouts/**/*.{vue,js,ts}'],
      rules: {
        'vue/multi-word-component-names': 'off', // Disable the rule for multi-word component names in pages and layouts
      },
    },
  ],
)
