<script lang="ts">
	import { createQuery } from "@tanstack/svelte-query";
	import { api } from "#lib/api/client.ts";

	// Temporary: proves CORS, generated types, and query wiring. Replaced by the login page.
	const health = createQuery(() => ({
		queryKey: ["health"],
		queryFn: async () => {
			const { data, error } = await api.GET("/health");
			if (error) {
				throw new Error("Server unavailable");
			}
			return data;
		},
		refetchInterval: 5000
	}));
</script>

<main class="p-8">
	{#if health.isPending}
		<p>Checking server...</p>
	{:else if health.isError}
		<p class="text-destructive">Error: {health.error.message}</p>
	{:else}
		<p>Server OK</p>
	{/if}
</main>
