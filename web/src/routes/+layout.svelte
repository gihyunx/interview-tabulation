<script lang="ts">
	import "./layout.css";
	import favicon from "#lib/assets/favicon.svg";
	import { startRealtime } from "#lib/realtime.svelte.ts";
	import { Toaster } from "#lib/components/ui/sonner/index.js";
	import { MutationCache, QueryClient, QueryClientProvider } from "@tanstack/svelte-query";
	import type { LayoutProps } from "./$types";

	let { children, data }: LayoutProps = $props();

	// Any successful change refetches what is on screen, so a screen is right even while the stream reconnects.
	const queryClient: QueryClient = new QueryClient({
		mutationCache: new MutationCache({ onSuccess: () => void queryClient.invalidateQueries() })
	});

	// One event stream per tab while logged in.
	const loggedIn = $derived(data.user !== null);
	$effect(() => (loggedIn ? startRealtime(queryClient) : undefined));
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<QueryClientProvider client={queryClient}>
	{@render children()}
</QueryClientProvider>
<Toaster richColors />
