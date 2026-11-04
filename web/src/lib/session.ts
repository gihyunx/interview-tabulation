import { redirect } from "@sveltejs/kit";
import { api } from "#lib/api/client.ts";
import type { components } from "#lib/api/schema.d.ts";

export type User = components["schemas"]["User"];

export function homeFor(user: User) {
	return user.role === "admin" ? "/admin" : "/judge";
}

export async function fetchMe(fetch: typeof globalThis.fetch): Promise<User | null> {
	const { data } = await api.GET("/v1/auth/me", { fetch });
	return data ?? null;
}

/** Redirects to where the user belongs; returns the user for pages they may see. */
export function guard(user: User | null, pathname: string): User | null {
	if (!user) {
		if (pathname !== "/login") {
			redirect(307, "/login");
		}
		return null;
	}
	const home = homeFor(user);
	if (pathname === "/login" || pathname === "/") {
		redirect(307, home);
	}
	if (!pathname.startsWith(home)) {
		redirect(307, home);
	}
	return user;
}
