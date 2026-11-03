<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
	import { BACKEND_URL } from "$app/env/public";
	import type { components } from "#lib/api/schema.d.ts";
	import StateMessage from "#lib/components/state-message.svelte";
	import { Badge } from "#lib/components/ui/badge/index.js";
	import { buttonVariants } from "#lib/components/ui/button/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import * as Table from "#lib/components/ui/table/index.js";
	import { resultsQuery } from "#lib/queries.ts";
	import { matchScore } from "#lib/search.ts";
	import { createQuery } from "@tanstack/svelte-query";
	import DetailSheet from "./detail-sheet.svelte";

	type Candidate = components["schemas"]["ResultCandidate"];

	// The graded interviews live in the URL. None given = the server's default.
	const requested = $derived(page.url.searchParams.getAll("interview"));
	const results = createQuery(() => resultsQuery(requested));

	const interviews = $derived(results.data?.interviews ?? []);
	const candidates = $derived(results.data?.candidates ?? []);
	const selected = $derived(interviews.filter((i) => i.selected));

	function toggle(id: string) {
		const current = selected.map((i) => i.id);
		if (current.includes(id) && current.length === 1) {
			return; // at least one interview is graded
		}
		const ids = current.includes(id) ? current.filter((x) => x !== id) : [...current, id];
		const qs = new URLSearchParams(ids.map((x) => ["interview", x]));
		// eslint-disable-next-line svelte/no-navigation-without-resolve -- the path is resolved, only the query is appended
		void goto(`${resolve("/admin/scores")}?${qs}`, { replaceState: true });
	}

	let search = $state("");
	const rows = $derived(
		candidates.filter((c) => matchScore(search, `${c.first_name} ${c.last_name}`) > 0)
	);

	let detailId = $state<string | null>(null);
	const detail = $derived(candidates.find((c) => c.id === detailId) ?? null);

	const totalOf = (c: Candidate, interviewId: string, judgeId?: string) =>
		(c.submissions ?? []).find(
			(s) => s.interview_id === interviewId && (judgeId === undefined || s.judge_id === judgeId)
		);

	const exportHref = $derived(
		`${BACKEND_URL}/v1/results.xlsx?${new URLSearchParams(selected.map((i) => ["interview", i.id]))}`
	);
	const modeLabel = (mode: string) => (mode === "panel" ? "Panel" : "1-on-1");
</script>

<svelte:head><title>Candidates & Scores</title></svelte:head>

<div class="mb-4 flex flex-wrap items-center justify-between gap-4">
	<h1 class="text-3xl font-bold">Candidates & Scores</h1>
	{#if results.data}
		<p class="text-sm text-muted-foreground" role="status">
			{results.data.fully_scored} of {candidates.length} candidates fully scored
		</p>
	{/if}
</div>

<div class="mb-4 flex flex-wrap items-center justify-between gap-4">
	<Input
		class="w-80"
		placeholder="Search candidates"
		aria-label="Search candidates"
		bind:value={search}
	/>
	{#if results.data}
		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- the API server, not an app route -->
		<a class={buttonVariants({ variant: "outline" })} href={exportHref} download>Export .xlsx</a>
	{/if}
</div>

<StateMessage pending={results.isPending} error={results.error} />

{#if results.data}
	<div class="mb-4 flex flex-wrap gap-2" role="group" aria-label="Graded interviews">
		{#each interviews as i (i.id)}
			<button
				type="button"
				aria-pressed={i.selected}
				class={buttonVariants({ variant: i.selected ? "default" : "outline", size: "sm" })}
				onclick={() => toggle(i.id)}
			>
				{i.title} ({modeLabel(i.mode)})
			</button>
		{/each}
		{#if interviews.length === 0}
			<p class="text-sm text-muted-foreground">No interviews yet.</p>
		{/if}
	</div>

	<div class="overflow-hidden rounded-xl border bg-card shadow-xs">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head rowspan={2}>#</Table.Head>
					<Table.Head rowspan={2}>Candidate</Table.Head>
					{#each selected as i (i.id)}
						<Table.Head
							colspan={i.mode === "panel" ? Math.max(i.judges?.length ?? 0, 1) : 1}
							class="border-l text-center"
						>
							{i.title}
						</Table.Head>
					{/each}
					<Table.Head rowspan={2} class="border-l text-right">Average</Table.Head>
				</Table.Row>
				<Table.Row>
					{#each selected as i (i.id)}
						{#if i.mode === "panel"}
							{#each i.judges ?? [] as j, n (j.judge_id)}
								<Table.Head class={["text-center font-normal", n === 0 && "border-l"]}
									>{j.name}</Table.Head
								>
							{:else}
								<Table.Head class="border-l text-center font-normal">No judges</Table.Head>
							{/each}
						{:else}
							<Table.Head class="border-l text-center font-normal">Interviewer</Table.Head>
						{/if}
					{/each}
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each rows as c, n (c.id)}
					<Table.Row class="cursor-pointer" onclick={() => (detailId = c.id)}>
						<Table.Cell class="text-muted-foreground">{n + 1}</Table.Cell>
						<Table.Cell>
							<button
								type="button"
								class="text-left font-semibold hover:underline"
								onclick={(e) => {
									e.stopPropagation();
									detailId = c.id;
								}}
							>
								{c.last_name}, {c.first_name}
							</button>
						</Table.Cell>
						{#each selected as i (i.id)}
							{#if i.mode === "panel"}
								{#each i.judges ?? [] as j, k (j.judge_id)}
									{@const s = totalOf(c, i.id, j.judge_id)}
									<Table.Cell class={["text-center", k === 0 && "border-l"]}
										>{s ? s.total : "-"}</Table.Cell
									>
								{:else}
									<Table.Cell class="border-l text-center">-</Table.Cell>
								{/each}
							{:else}
								{@const s = totalOf(c, i.id)}
								<Table.Cell class="border-l text-center">
									{#if s}
										{s.total}
										<span class="block text-xs text-muted-foreground">{s.judge_name}</span>
									{:else}
										-
									{/if}
								</Table.Cell>
							{/if}
						{/each}
						<Table.Cell class="border-l text-right font-bold">
							{#if c.complete && c.average != null}
								{c.average.toFixed(2)}
							{:else}
								<Badge variant="secondary">Pending</Badge>
							{/if}
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row>
						<Table.Cell colspan={3 + selected.length} class="text-center text-muted-foreground">
							{candidates.length === 0 ? "No candidates yet." : "No candidates match your search."}
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<p class="mt-3 text-sm text-muted-foreground">
		All scores are out of 100. The average appears once all scores are in.
	</p>
{/if}

<DetailSheet candidate={detail} {selected} onclose={() => (detailId = null)} />
