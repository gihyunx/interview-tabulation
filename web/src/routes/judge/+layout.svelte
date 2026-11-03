<script lang="ts">
	import { resolve } from "$app/paths";
	import ConnectionPill from "#lib/components/connection-pill.svelte";
	import LogoutButton from "#lib/components/logout-button.svelte";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();
</script>

<!-- Full viewport height so the scoring screen can fill it (the candidate list takes what is left). -->
<div class="flex min-h-dvh flex-col lg:h-dvh">
	<header class="flex items-center justify-between border-b bg-card px-6 py-3">
		<a href={resolve("/judge")} class="text-lg font-extrabold tracking-tight text-primary"
			>Interview Tabulation</a
		>
		<div class="flex items-center gap-4">
			<ConnectionPill />
			<span class="text-sm font-medium">{data.user?.first_name} {data.user?.last_name}</span>
			<LogoutButton />
		</div>
	</header>
	<main class="flex min-h-0 flex-1 flex-col p-6">
		{@render children()}
	</main>
</div>
