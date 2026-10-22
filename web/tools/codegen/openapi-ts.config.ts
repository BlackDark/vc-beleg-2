import { defineConfig } from "@hey-api/openapi-ts";

// Generates web/src/api/gen from the Go server's OpenAPI document
// (go run ./cmd/mahlzeit openapi > web/openapi.json). Never edit the output.
export default defineConfig({
  input: "../../openapi.json",
  output: { path: "../../src/api/gen" },
  plugins: [
    "@hey-api/typescript",
    { name: "@hey-api/client-fetch", baseUrl: "" },
    "zod",
    { name: "@hey-api/sdk", validator: { response: "zod" } },
    "@tanstack/react-query",
  ],
});
