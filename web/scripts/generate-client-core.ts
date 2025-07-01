import { createClient, defaultPlugins } from "@hey-api/openapi-ts";

createClient({
  input: "./../services/core/docs/open-api-specs.yaml",
  output: {
    path: "./app/clients/core",
    lint: "eslint",
    indexFile: false,
  },
  plugins: [
    ...defaultPlugins,
    {
      name: "@hey-api/typescript",
      readableNameBuilder: "{{name}}",
      writableNameBuilder: "{{name}}",
    // ...custom options
    },
    { name: "@hey-api/client-nuxt", runtimeConfigPath: "./app/clients/configuration/core" },
  ],
});
