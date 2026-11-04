import { redirect } from "@sveltejs/kit";
import { api } from "#lib/api/client.ts";
import type { PageLoad } from "./$types";

// A judge with one interview goes straight to it.
export const load: PageLoad = async ({ fetch }) => {
	const { data } = await api.GET("/v1/me/interviews", { fetch });
	if (data?.length === 1) {
		redirect(307, `/judge/${data[0].id}`);
	}
};
