<script lang="ts">
	import { api, errorMessage } from "#lib/api/client.ts";
	import type { components } from "#lib/api/schema.d.ts";
	import ConfirmDialog from "#lib/components/confirm-dialog.svelte";
	import StateMessage from "#lib/components/state-message.svelte";
	import { Badge } from "#lib/components/ui/badge/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Label } from "#lib/components/ui/label/index.js";
	import { Switch } from "#lib/components/ui/switch/index.js";
	import { Textarea } from "#lib/components/ui/textarea/index.js";
	import { interviewsQuery, rubricsQuery } from "#lib/queries.ts";
	import { untrack } from "svelte";
	import { createMutation, createQuery, useQueryClient } from "@tanstack/svelte-query";
	import GripVertical from "@lucide/svelte/icons/grip-vertical";
	import X from "@lucide/svelte/icons/x";
	import { dndzone, type DndEvent } from "svelte-dnd-action";
	import { flip } from "svelte/animate";
	import { toast } from "svelte-sonner";

	type Rubric = components["schemas"]["Rubric"];
	// id is the dnd key; cid is the server id of an existing criterion.
	type Row = { id: string; cid?: string; title: string; description: string; max_points: number };

	const queryClient = useQueryClient();
	const rubrics = createQuery(rubricsQuery);
	const interviews = createQuery(interviewsQuery);

	const NEW = "new";
	let selectedId = $state<string | null>(null);
	let name = $state("");
	let rows = $state<Row[]>([]);
	let baseline = $state("");
	let dragDisabled = $state(true);
	let deleteOpen = $state(false);

	const snapshot = () =>
		JSON.stringify({ name, rows: rows.map((r) => [r.cid, r.title, r.description, r.max_points]) });
	const dirty = $derived(baseline !== "" && snapshot() !== baseline);

	const current = $derived(
		selectedId === NEW ? undefined : rubrics.data?.find((r) => r.id === selectedId)
	);
	const locked = $derived(current?.locked ?? false);
	const total = $derived(rows.reduce((sum, r) => sum + (Number(r.max_points) || 0), 0));
	const complete = $derived(
		name.trim() !== "" && rows.length > 0 && rows.every((r) => r.title.trim() !== "")
	);

	function load(r: Rubric) {
		name = r.name;
		rows = (r.criteria ?? []).map((c) => ({
			id: c.id,
			cid: c.id,
			title: c.title,
			description: c.description,
			max_points: c.max_points
		}));
		baseline = snapshot();
	}

	// Pick the first rubric on arrival, and follow server changes unless there are unsaved edits.
	// Only the fetched data triggers this; the body is untracked because it writes the form state.
	$effect(() => {
		const data = rubrics.data;
		if (data) {
			untrack(() => sync(data));
		}
	});

	function sync(data: Rubric[]) {
		if (selectedId === null || (selectedId !== NEW && !data.some((r) => r.id === selectedId))) {
			if (data.length === 0) {
				startNew([]);
			} else {
				selectedId = data[0].id;
				load(data[0]);
			}
			return;
		}
		const shown = data.find((r) => r.id === selectedId);
		if (shown && !dirty) {
			load(shown);
		}
	}

	function select(id: string) {
		if (id === selectedId) {
			return;
		}
		if (dirty && !confirm("Discard your unsaved changes?")) {
			return;
		}
		selectedId = id;
		const r = rubrics.data?.find((x) => x.id === id);
		if (r) {
			load(r);
		}
	}

	function startNew(copyFrom: Row[]) {
		selectedId = NEW;
		name = "";
		rows = copyFrom.map((r) => ({ ...r, id: crypto.randomUUID(), cid: undefined }));
		baseline = snapshot();
	}

	function newRubric() {
		if (dirty && !confirm("Discard your unsaved changes?")) {
			return;
		}
		startNew(rows);
		name = current ? `Copy of ${current.name}` : "";
	}

	function addCriterion() {
		rows.push({ id: crypto.randomUUID(), title: "", description: "", max_points: 0 });
	}

	function consider(e: CustomEvent<DndEvent<Row>>) {
		rows = e.detail.items;
	}
	function finalize(e: CustomEvent<DndEvent<Row>>) {
		rows = e.detail.items;
		dragDisabled = true;
	}
	function handleKey(e: KeyboardEvent) {
		if (e.key === "Enter" || e.key === " ") {
			e.preventDefault();
			dragDisabled = !dragDisabled;
		}
	}

	const save = createMutation(() => ({
		mutationFn: async () => {
			const body = {
				name: name.trim(),
				criteria: rows.map((r) => ({
					id: r.cid,
					title: r.title.trim(),
					description: r.description,
					max_points: Number(r.max_points)
				}))
			};
			const { data, error } =
				selectedId === NEW
					? await api.POST("/v1/rubrics", { body })
					: await api.PATCH("/v1/rubrics/{id}", {
							params: { path: { id: selectedId! } },
							body
						});
			if (error) {
				throw new Error(errorMessage(error));
			}
			return data;
		},
		onSuccess: async (saved) => {
			toast.success("Rubric saved.");
			selectedId = saved.id;
			load(saved);
			await queryClient.invalidateQueries({ queryKey: ["rubrics"] });
		},
		onError: (e) => toast.error(e.message)
	}));

	const remove = createMutation(() => ({
		mutationFn: async () => {
			const { error } = await api.DELETE("/v1/rubrics/{id}", {
				params: { path: { id: selectedId! } }
			});
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => {
			toast.success("Rubric deleted.");
			selectedId = null;
			baseline = "";
		},
		onError: (e) => toast.error(e.message)
	}));

	const apply = createMutation(() => ({
		mutationFn: async (interviewId: string) => {
			const { error } = await api.PATCH("/v1/interviews/{id}", {
				params: { path: { id: interviewId } },
				body: { rubric_id: selectedId! }
			});
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => toast.success("Interview now uses this rubric."),
		onError: (e) => toast.error(e.message)
	}));

	const modeLabel = (mode: string) => (mode === "panel" ? "Panel" : "1-on-1");
	const left = $derived(100 - total);
	const deleteBlock = $derived(
		current?.used_by?.length
			? `Used by ${current.used_by.join(", ")}`
			: current?.locked
				? "Already has scores"
				: ""
	);
</script>

<svelte:head><title>Rubrics</title></svelte:head>

<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
	<h1 class="text-3xl font-bold">Rubrics</h1>
	<div class="flex flex-wrap gap-2">
		{#each interviews.data ?? [] as i (i.id)}
			<Badge variant="secondary">{modeLabel(i.mode)}: {i.rubric_name}</Badge>
		{/each}
	</div>
</div>

<StateMessage pending={rubrics.isPending} error={rubrics.error} />

{#if rubrics.data}
	<div class="mb-6 flex flex-wrap gap-2" role="group" aria-label="Rubrics">
		{#each rubrics.data as r (r.id)}
			<Button
				variant={r.id === selectedId ? "default" : "outline"}
				aria-pressed={r.id === selectedId}
				onclick={() => select(r.id)}
			>
				{r.name}
			</Button>
		{/each}
		{#if selectedId === NEW}
			<Button aria-pressed="true">{name || "New rubric"}</Button>
		{/if}
		<Button variant="ghost" onclick={newRubric}>+ New rubric</Button>
	</div>

	<div class="max-w-3xl space-y-6">
		<div class="space-y-2">
			<Label for="rubric-name">Rubric name</Label>
			<Input id="rubric-name" bind:value={name} />
		</div>

		<div class="space-y-2">
			<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">Applies to</p>
			<div class="flex flex-wrap gap-4">
				{#each interviews.data ?? [] as i (i.id)}
					{@const using = i.rubric_id === selectedId}
					<label class="flex items-center gap-2 text-sm">
						<Switch
							checked={using}
							disabled={using || selectedId === NEW || i.has_scores || apply.isPending}
							onCheckedChange={(on) => on && apply.mutate(i.id)}
						/>
						{i.title}
					</label>
				{/each}
			</div>
			<p class="text-sm text-muted-foreground">
				Every interview has exactly one rubric. Turn a toggle on to move that interview to this
				rubric. Interviews that already have scores cannot change rubric.
			</p>
		</div>

		{#if locked}
			<p class="rounded-md bg-warning p-3 text-sm text-warning-foreground" role="note">
				Scores already use this rubric, so criteria and points are locked. You can still edit
				titles, descriptions and order.
			</p>
		{/if}

		<div
			class="space-y-3"
			use:dndzone={{ items: rows, dragDisabled, flipDurationMs: 150, dropTargetStyle: {} }}
			onconsider={consider}
			onfinalize={finalize}
		>
			{#each rows as row, n (row.id)}
				<div
					class="flex items-start gap-3 rounded-xl border bg-card p-3 shadow-xs"
					animate:flip={{ duration: 150 }}
				>
					<button
						type="button"
						class="mt-2 cursor-grab text-muted-foreground"
						aria-label={`Reorder criterion ${n + 1}. Press Enter to pick up, arrow keys to move.`}
						tabindex={dragDisabled ? 0 : -1}
						onmousedown={() => (dragDisabled = false)}
						ontouchstart={() => (dragDisabled = false)}
						onkeydown={handleKey}
					>
						<GripVertical class="size-5" />
					</button>
					<span class="mt-2 w-6 text-sm font-semibold text-muted-foreground">{n + 1}</span>
					<div class="flex-1 space-y-2">
						<Input
							aria-label={`Criterion ${n + 1} title`}
							placeholder="Title"
							bind:value={row.title}
						/>
						<Textarea
							aria-label={`Criterion ${n + 1} description`}
							placeholder="Description"
							rows={2}
							bind:value={row.description}
						/>
					</div>
					<div class="w-24">
						<Input
							type="number"
							min={1}
							max={100}
							aria-label={`Criterion ${n + 1} points`}
							disabled={locked}
							bind:value={row.max_points}
						/>
					</div>
					<Button
						variant="ghost"
						size="icon"
						aria-label={`Remove criterion ${n + 1}`}
						disabled={locked}
						onclick={() => (rows = rows.filter((r) => r.id !== row.id))}
					>
						<X class="size-4" />
					</Button>
				</div>
			{/each}
		</div>

		{#if !locked}
			<Button variant="outline" onclick={addCriterion}>+ Add criterion</Button>
		{/if}

		<div class="flex flex-wrap items-center justify-between gap-4 border-t pt-4">
			<div>
				<p class="text-sm font-semibold tracking-wide text-muted-foreground uppercase">Total</p>
				<p class={["text-3xl font-bold", total !== 100 && "text-destructive"]}>
					{total} <span class="text-base font-medium text-muted-foreground">/ 100</span>
				</p>
				{#if left > 0}
					<p class="text-sm text-muted-foreground">{left} points left to assign</p>
				{:else if left < 0}
					<p class="text-sm text-destructive">{-left} points over</p>
				{/if}
			</div>
			<div class="flex gap-2">
				{#if selectedId !== NEW}
					<Button
						variant="outline"
						disabled={deleteBlock !== ""}
						title={deleteBlock}
						onclick={() => (deleteOpen = true)}
					>
						Delete{deleteBlock ? ` (${deleteBlock})` : ""}
					</Button>
				{/if}
				<Button
					disabled={total !== 100 || !complete || save.isPending}
					onclick={() => save.mutate()}
				>
					Save rubric
				</Button>
			</div>
		</div>
	</div>
{/if}

<ConfirmDialog
	bind:open={deleteOpen}
	title={`Delete ${current?.name}?`}
	description="This removes the rubric and its criteria."
	confirmLabel="Delete"
	onconfirm={() => remove.mutate()}
/>
