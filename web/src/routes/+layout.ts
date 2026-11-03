import { fetchMe, guard } from "#lib/session.ts";
import type { LayoutLoad } from "./$types";

export const ssr = false;

export const load: LayoutLoad = async ({ fetch, url }) => {
	const user = guard(await fetchMe(fetch), url.pathname);
	return { user };
};
