import createClient, { type Middleware } from "openapi-fetch";
import { goto } from "$app/navigation";
import { resolve } from "$app/paths";
import { BACKEND_URL } from "$app/env/public";
import type { paths } from "./schema";

export const api = createClient<paths>({ baseUrl: BACKEND_URL, credentials: "include" });

// An expired session mid-use sends the user back to login. The auth endpoints
// handle their own 401s (wrong password, not logged in yet).
const sessionExpired: Middleware = {
	onResponse({ response }) {
		if (response.status === 401 && !new URL(response.url).pathname.startsWith("/v1/auth/")) {
			void goto(resolve("/login"));
		}
	}
};
api.use(sessionExpired);

/** Readable message from a Huma error body, or a fallback. */
export function errorMessage(error: unknown, fallback = "Something went wrong."): string {
	if (error && typeof error === "object" && "detail" in error && typeof error.detail === "string") {
		return error.detail;
	}
	return fallback;
}

/** An error response from the API, with its status so callers can decide whether to retry. */
export class ApiError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
	}
}
