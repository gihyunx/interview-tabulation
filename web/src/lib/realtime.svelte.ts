import { BACKEND_URL } from "$app/env/public";
import type { QueryClient } from "@tanstack/svelte-query";

type Topic = "scores" | "candidates" | "rubrics" | "interviews" | "users";

// Events are hints: they only say what to refetch.
const keys: Record<Topic, string[]> = {
	scores: ["board", "results"],
	candidates: ["candidates", "board", "results"],
	rubrics: ["rubrics", "interviews", "my-interviews", "results"],
	interviews: ["interviews", "my-interviews", "board", "results"],
	users: ["users", "interviews"]
};

export const connection = $state({ live: false });

/** Opens the event stream. Returns a function that closes it. */
export function startRealtime(queryClient: QueryClient): () => void {
	const source = new EventSource(`${BACKEND_URL}/v1/events`, { withCredentials: true });

	source.onopen = () => {
		connection.live = true;
		// Anything may have changed while we were away.
		void queryClient.invalidateQueries();
	};
	source.onerror = () => {
		connection.live = false; // EventSource reconnects by itself
	};
	source.onmessage = (e) => {
		try {
			const { topic } = JSON.parse(e.data) as { topic: Topic };
			for (const key of keys[topic] ?? []) {
				void queryClient.invalidateQueries({ queryKey: [key] });
			}
		} catch {
			void queryClient.invalidateQueries();
		}
	};

	return () => {
		source.close();
		connection.live = false;
	};
}
