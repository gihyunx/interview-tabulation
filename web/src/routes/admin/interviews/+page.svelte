<script lang="ts">
	import { api, errorMessage } from "#lib/api/client.ts";
	import type { components } from "#lib/api/schema.d.ts";
	import ConfirmDialog from "#lib/components/confirm-dialog.svelte";
	import { latest } from "#lib/latest.svelte.ts";
	import StateMessage from "#lib/components/state-message.svelte";
	import { Badge } from "#lib/components/ui/badge/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { candidatesQuery, interviewsQuery } from "#lib/queries.ts";
	import { createMutation, createQuery, useQueryClient } from "@tanstack/svelte-query";
	import GripVertical from "@lucide/svelte/icons/grip-vertical";
	import { dndzone, type DndEvent } from "svelte-dnd-action";
	import { flip } from "svelte/animate";
	import { toast } from "svelte-sonner";
	import InterviewDialog from "./interview-dialog.svelte";
	import JudgesDialog from "./judges-dialog.svelte";

	type Interview = components["schemas"]["Interview"];

	const queryClient = useQueryClient();
	const interviews = createQuery(interviewsQuery);
	const candidates = createQuery(candidatesQuery);

	// Follows the server, but is assigned on a drop so the new order shows at once.
	let items = $derived(interviews.data ?? []);
	let dragDisabled = $state(true);

	let editing = $state<{ interview?: Interview } | null>(null);
	let assigning = $state<Interview | null>(null);
	let deleting = $state<Interview | null>(null);
	const shownDeleting = latest(() => deleting);

	const reorder = createMutation(() => ({
		mutationFn: async (ids: string[]) => {
			const { error } = await api.PUT("/v1/interviews/order", { body: { ids } });
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onError: (e) => toast.error(e.message),
		onSettled: () => queryClient.invalidateQueries({ queryKey: ["interviews"] })
	}));

	function consider(e: CustomEvent<DndEvent<Interview>>) {
		items = e.detail.items;
	}
	function finalize(e: CustomEvent<DndEvent<Interview>>) {
		items = e.detail.items;
		dragDisabled = true;
		reorder.mutate(items.map((i) => i.id));
	}
	function handleKey(e: KeyboardEvent) {
		if (e.key === "Enter" || e.key === " ") {
			e.preventDefault();
			dragDisabled = !dragDisabled;
		}
	}

	const remove = createMutation(() => ({
		mutationFn: async (i: Interview) => {
			const { error } = await api.DELETE("/v1/interviews/{id}", { params: { path: { id: i.id } } });
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => toast.success("Interview deleted."),
		onError: (e) => toast.error(e.message),
		onSettled: () => queryClient.invalidateQueries({ queryKey: ["interviews"] })
	}));

	const candidateCount = $derived(candidates.data?.length ?? 0);
	const modeLabel = (mode: string) => (mode === "panel" ? "Panel" : "1-on-1");
</script>

<svelte:head><title>Interviews</title></svelte:head>

<div class="mb-6 flex items-center justify-between">
	<h1 class="text-3xl font-bold">Interviews</h1>
	<Button onclick={() => (editing = {})}>New interview</Button>
</div>

<StateMessage
	pending={interviews.isPending}
	error={interviews.error}
	empty={interviews.data?.length === 0
		? "No interviews yet. Create a rubric first, then an interview that uses it."
		: undefined}
/>

<div
	class="max-w-4xl space-y-3"
	use:dndzone={{ items, dragDisabled, flipDurationMs: 150, dropTargetStyle: {} }}
	onconsider={consider}
	onfinalize={finalize}
>
	{#each items as i (i.id)}
		<section class="rounded-xl border bg-card p-4 shadow-xs" animate:flip={{ duration: 150 }}>
			<div class="flex items-start gap-3">
				<button
					type="button"
					class="mt-1 cursor-grab text-muted-foreground"
					aria-label={`Reorder ${i.title}. Press Enter to pick up, arrow keys to move.`}
					tabindex={dragDisabled ? 0 : -1}
					onmousedown={() => (dragDisabled = false)}
					ontouchstart={() => (dragDisabled = false)}
					onkeydown={handleKey}
				>
					<GripVertical class="size-5" />
				</button>
				<div class="flex-1">
					<div class="flex flex-wrap items-center gap-2">
						<h2 class="text-lg font-bold">{i.title}</h2>
						<Badge variant="secondary">{modeLabel(i.mode)}</Badge>
						<span class="text-sm text-muted-foreground">Rubric: {i.rubric_name}</span>
					</div>
					{#if i.description}
						<p class="text-sm text-muted-foreground">{i.description}</p>
					{/if}

					<ul class="mt-3 space-y-1 text-sm">
						{#each i.judges ?? [] as j (j.judge_id)}
							<li class="flex justify-between gap-4 sm:w-96">
								<span>{j.first_name} {j.last_name}</span>
								<span class="text-muted-foreground">{j.scored} of {candidateCount} scored</span>
							</li>
						{:else}
							<li class="text-muted-foreground">No judges assigned.</li>
						{/each}
					</ul>
				</div>
				<div class="flex flex-wrap justify-end gap-2">
					<Button variant="outline" size="sm" onclick={() => (assigning = i)}>Judges</Button>
					<Button variant="outline" size="sm" onclick={() => (editing = { interview: i })}
						>Edit</Button
					>
					<Button
						variant="outline"
						size="sm"
						disabled={i.has_scores}
						title={i.has_scores ? "Already has scores" : ""}
						onclick={() => (deleting = i)}
					>
						Delete
					</Button>
				</div>
			</div>
		</section>
	{/each}
</div>

<InterviewDialog bind:state={editing} />
<JudgesDialog bind:interview={assigning} />
<ConfirmDialog
	bind:open={() => deleting !== null, (v) => !v && (deleting = null)}
	title={`Delete ${shownDeleting.current?.title}?`}
	description="This also removes its judge assignments. Interviews that already have scores cannot be deleted."
	confirmLabel="Delete"
	onconfirm={() => {
		if (deleting) {
			remove.mutate(deleting);
		}
		deleting = null;
	}}
/>
