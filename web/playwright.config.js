import { defineConfig } from "@playwright/test";
import { testOutputDir } from "./tests/e2e/paths.js";

export default defineConfig({
  testDir: "./tests/e2e",
  outputDir: testOutputDir,
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:4179",
    headless: true,
    channel: process.env.REPROKIT_TEST_BROWSER || undefined,
    screenshot: "only-on-failure",
  },
  webServer: {
    command: "node server.mjs",
    url: "http://127.0.0.1:4179",
    env: {
      REPROKIT_WEB_PORT: "4179",
      REPROKIT_RUNNER_ORIGIN: "http://127.0.0.1:9",
      REPROKIT_RUNNER_TOKEN: "test_token_for_local_session",
    },
    reuseExistingServer: false,
  },
});
