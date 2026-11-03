<script lang="ts">
	import { resolve } from "$app/paths";
	import StateMessage from "#lib/components/state-message.svelte";
	import { Badge } from "#lib/components/ui/badge/index.js";
	import { myInterviewsQuery } from "#lib/queries.ts";
	import { createQuery } from "@tanstack/svelte-query";

	const interviews = createQuery(myInterviewsQuery);
</script>

<svelte:head><title>Your interviews</title></svelte:head>

<h1 class="mb-6 text-3xl font-bold">Your interviews</h1>

<StateMessage
	pending={interviews.isPending}
	error={interviews.error}
	empty={interviews.data?.length === 0
		? "You are not assigned to any interview yet. Ask the admin."
		: undefined}
/>

<ul class="grid max-w-3xl gap-4 sm:grid-cols-2">
	{#each interviews.data ?? [] as i (i.id)}
		<li>
			<a
				href={resolve("/judge/[interviewId]", { interviewId: i.id })}
				class="block rounded-xl border bg-card p-6 shadow-xs transition-shadow hover:border-ring hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
			>
				<Badge variant="secondary"
					>{i.mode === "panel" ? "Panel interview" : "1-on-1 interview"}</Badge
				>
				<h2 class="mt-2 text-xl font-bold">{i.title}</h2>
				{#if i.description}<p class="text-sm text-muted-foreground">{i.description}</p>{/if}
			</a>
		</li>
	{/each}
</ul>
