import { defineConfig } from "@playwright/test";

const EDGE_URL = process.env.PALADIN_EDGE_URL || "http://localhost:9002";
const INGEST_URL = process.env.PALADIN_INGEST_URL || "http://localhost:9001";
const AUTH_URL = process.env.PALADIN_AUTH_URL || "http://localhost:9003";
const HUB_URL = process.env.PALADIN_HUB_URL || "http://localhost:8082";

export default defineConfig({
  testDir: "./tests",
  timeout: 30_000,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: EDGE_URL,
    extraHTTPHeaders: {
      "Content-Type": "application/json",
    },
  },
  projects: [
    {
      name: "api",
      testMatch: /.*\.api\.ts$/,
      use: {
        baseURL: EDGE_URL,
      },
    },
    {
      name: "ingest",
      testMatch: /.*\.ingest\.ts$/,
      use: {
        baseURL: INGEST_URL,
      },
    },
    {
      name: "auth",
      testMatch: /.*\.auth\.ts$/,
      use: {
        baseURL: AUTH_URL,
      },
    },
    {
      name: "hub",
      testMatch: /.*\.hub\.ts$/,
      use: {
        baseURL: HUB_URL,
      },
    },
  ],
});
