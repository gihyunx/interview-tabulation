import { execFileSync } from "node:child_process";
import { e2eEnv } from "../playwright.config";

// The API has migrated the fresh database by the time this runs.
export default function globalSetup() {
	execFileSync("go", ["run", "./cmd/seed"], {
		cwd: "../server",
		env: { ...process.env, ...e2eEnv },
		stdio: "inherit"
	});
}
