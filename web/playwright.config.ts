import { defineConfig } from "@playwright/test";

// A separate stack on other ports with its own database, so e2e never touches dev data.
const dbUser = process.env.DB_USER ?? "postgres";
const dbPassword = process.env.DB_PASSWORD ?? "postgres";
const dbHost = process.env.DB_HOST ?? "localhost";
const dbPort = process.env.DB_PORT ?? "5432";

export const e2eEnv = {
	DATABASE_URL: `postgresql://${dbUser}:${dbPassword}@${dbHost}:${dbPort}/tabulation_e2e`,
	PORT: "8081",
	ALLOWED_ORIGINS: "http://localhost:5174",
	JWT_SECRET: "e2e-secret",
	ADMIN_USERNAME: "admin",
	ADMIN_PASSWORD: "e2e-password",
	ADMIN_FIRST_NAME: "Ada",
	ADMIN_LAST_NAME: "Admin",
	APP_ENV: "test"
};

export default defineConfig({
	testDir: "e2e",
	globalSetup: "./e2e/global-setup.ts",
	fullyParallel: false,
	workers: 1,
	retries: 0,
	timeout: 120_000,
	expect: { timeout: 10_000 },
	use: {
		baseURL: "http://localhost:5174",
		actionTimeout: 15_000,
		trace: "retain-on-failure",
		screenshot: "only-on-failure"
	},
	webServer: [
		{
			command: "./e2e/start-server.sh",
			url: "http://localhost:8081/health",
			env: e2eEnv,
			reuseExistingServer: false,
			timeout: 120_000
		},
		{
			command: "pnpm vite dev --port 5174 --strictPort",
			url: "http://localhost:5174/login",
			env: { BACKEND_URL: "http://localhost:8081" },
			reuseExistingServer: false,
			timeout: 120_000
		}
	]
});
