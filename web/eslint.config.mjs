import antfu from "@antfu/eslint-config";

export default antfu(
  {
    vue: true,
    stylistic: {
      indent: 2, // 4, or 'tab'
      quotes: "double", // or 'double'
      semi: true,
      printWidth: 80,
    },
  },
  {
    files: ["**/*.vue"],
    rules: { "vue/max-attributes-per-line":
      [
        "warn",
        {
          multiline: 1,
          singleline: 1,
        },
      ] },
  },
  // {
  //   files: ["**/*.ts"],
  //   rules: { "style/max-len":
  //     [
  //       "warn",
  //       {
  //         code: 120,
  //       },
  //     ] },
  // },
  {
    files: ["**/*.ts", "**/*.vue"],
    rules: {
      "vue/block-order": "off",
      "node/prefer-global/process": "off",
    },
  },
);
