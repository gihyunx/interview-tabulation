<script lang="ts">
	import "./layout.css";
	import favicon from "#lib/assets/favicon.svg";
	import { QueryClient, QueryClientProvider } from "@tanstack/svelte-query";
	import type { LayoutProps } from "./$types";

	let { children }: LayoutProps = $props();

	const queryClient = new QueryClient();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<QueryClientProvider client={queryClient}>
	{@render children()}
</QueryClientProvider>
