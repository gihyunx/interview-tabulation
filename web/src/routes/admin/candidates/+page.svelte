<script lang="ts">
	import { api, errorMessage } from "#lib/api/client.ts";
	import ConfirmDialog from "#lib/components/confirm-dialog.svelte";
	import { latest } from "#lib/latest.svelte.ts";
	import StateMessage from "#lib/components/state-message.svelte";
	import { Checkbox } from "#lib/components/ui/checkbox/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Label } from "#lib/components/ui/label/index.js";
	import * as Table from "#lib/components/ui/table/index.js";
	import { candidatesQuery } from "#lib/queries.ts";
	import { matchScore } from "#lib/search.ts";
	import type { components } from "#lib/api/schema.d.ts";
	import { createMutation, createQuery } from "@tanstack/svelte-query";
	import { toast } from "svelte-sonner";
	import ImportDialog from "./import-dialog.svelte";

	type Candidate = components["schemas"]["Candidate"];

	const candidates = createQuery(candidatesQuery);

	let search = $state("");
	const shown = $derived(
		(candidates.data ?? []).filter(
			(c) => matchScore(search, `${c.number} ${c.first_name} ${c.last_name}`) > 0
		)
	);

	// null = closed; {} = create; { candidate } = edit.
	let editing = $state<{ candidate?: Candidate } | null>(null);
	const shownEditing = latest(() => editing);
	let number = $state<number | null>(null);
	let firstName = $state("");
	let lastName = $state("");
	let deleting = $state<Candidate | null>(null);
	const shownDeleting = latest(() => deleting);
	let importing = $state(false);
	let renumbering = $state(false);

	let picked = $state<string[]>([]);
	// Ignore ids that no longer exist (deleted elsewhere).
	const selected = $derived(
		(candidates.data ?? []).filter((c) => picked.includes(c.id)).map((c) => c.id)
	);
	const allShown = $derived(shown.length > 0 && shown.every((c) => picked.includes(c.id)));
	let bulkDeleting = $state(false);

	function toggle(id: string, on: boolean) {
		picked = on ? [...picked, id] : picked.filter((p) => p !== id);
	}
	function toggleAll(on: boolean) {
		const ids = new Set(shown.map((c) => c.id));
		picked = on ? [...new Set([...picked, ...ids])] : picked.filter((p) => !ids.has(p));
	}

	function openForm(candidate?: Candidate) {
		number = candidate?.number ?? null;
		firstName = candidate?.first_name ?? "";
		lastName = candidate?.last_name ?? "";
		save.reset();
		editing = { candidate };
	}

	const save = createMutation(() => ({
		mutationFn: async () => {
			const body = { number: number ?? undefined, first_name: firstName, last_name: lastName };
			const c = editing?.candidate;
			const { error } = c
				? await api.PATCH("/v1/candidates/{id}", { params: { path: { id: c.id } }, body })
				: await api.POST("/v1/candidates", { body });
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => {
			toast.success(editing?.candidate ? "Candidate updated." : "Candidate added.");
			editing = null;
		}
	}));

	const renumber = createMutation(() => ({
		mutationFn: async () => {
			const { error } = await api.POST("/v1/candidates/renumber");
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => toast.success("Candidates renumbered."),
		onError: (e) => toast.error(e.message)
	}));

	const removeMany = createMutation(() => ({
		mutationFn: async (ids: string[]) => {
			const results = await Promise.all(
				ids.map((id) => api.DELETE("/v1/candidates/{id}", { params: { path: { id } } }))
			);
			const failed = results.filter((r) => r.error).length;
			return { deleted: ids.length - failed, failed };
		},
		onSuccess: ({ deleted, failed }) => {
			picked = [];
			if (failed === 0) {
				toast.success(`Deleted ${deleted} candidates.`);
			} else {
				// Candidates with scores are refused by the server.
				toast.error(`Deleted ${deleted}. ${failed} could not be deleted (they may have scores).`);
			}
		},
		onError: (e) => toast.error(e.message)
	}));

	const remove = createMutation(() => ({
		mutationFn: async (c: Candidate) => {
			const { error } = await api.DELETE("/v1/candidates/{id}", { params: { path: { id: c.id } } });
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => toast.success("Candidate deleted."),
		onError: (e) => toast.error(e.message)
	}));
</script>

<svelte:head><title>Candidates</title></svelte:head>

<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
	<h1 class="text-3xl font-bold">Candidates</h1>
	<div class="flex flex-wrap gap-2">
		{#if selected.length > 0}
			<Button variant="destructive" onclick={() => (bulkDeleting = true)}>
				Delete selected ({selected.length})
			</Button>
		{/if}
		<Button variant="outline" disabled={renumber.isPending} onclick={() => (renumbering = true)}>
			Renumber
		</Button>
		<Button variant="outline" onclick={() => (importing = true)}>Import CSV</Button>
		<Button onclick={() => openForm()}>New candidate</Button>
	</div>
</div>

<Input
	class="mb-4 sm:w-80"
	placeholder="Search by number or name"
	aria-label="Search candidates"
	bind:value={search}
/>

<StateMessage pending={candidates.isPending} error={candidates.error} />
{#if candidates.data}
	<p class="mb-2 text-sm text-muted-foreground">
		{shown.length === candidates.data.length
			? `${shown.length} candidates`
			: `${shown.length} of ${candidates.data.length} match`}
	</p>
	<div class="overflow-hidden rounded-xl border bg-card shadow-xs">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head class="w-10">
						<Checkbox
							aria-label="Select all shown candidates"
							checked={allShown}
							onCheckedChange={(v) => toggleAll(v === true)}
						/>
					</Table.Head>
					<Table.Head class="w-24">No.</Table.Head>
					<Table.Head>Name</Table.Head>
					<Table.Head class="text-right">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each shown as c (c.id)}
					<Table.Row>
						<Table.Cell>
							<Checkbox
								aria-label={`Select candidate ${c.number}`}
								checked={picked.includes(c.id)}
								onCheckedChange={(v) => toggle(c.id, v === true)}
							/>
						</Table.Cell>
						<Table.Cell class="text-lg font-bold tabular-nums">{c.number}</Table.Cell>
						<Table.Cell class="text-muted-foreground">{c.last_name}, {c.first_name}</Table.Cell>
						<Table.Cell class="space-x-2 text-right">
							<Button variant="outline" size="sm" onclick={() => openForm(c)}>Edit</Button>
							<Button variant="outline" size="sm" onclick={() => (deleting = c)}>Delete</Button>
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row>
						<Table.Cell colspan={4} class="text-center text-muted-foreground">
							{candidates.data.length === 0
								? "No candidates yet. Add one or import a CSV."
								: "No candidates match your search."}
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}

<Dialog.Root open={editing !== null} onOpenChange={(open) => !open && (editing = null)}>
	<Dialog.Content>
		<form
			class="space-y-4"
			onsubmit={(e) => {
				e.preventDefault();
				save.mutate();
			}}
		>
			<Dialog.Header>
				<Dialog.Title
					>{shownEditing.current?.candidate ? "Edit candidate" : "New candidate"}</Dialog.Title
				>
				<Dialog.Description>The number and name judges will see.</Dialog.Description>
			</Dialog.Header>
			<div class="space-y-2">
				<Label for="c-number">Candidate number</Label>
				<Input
					id="c-number"
					type="number"
					min={1}
					required={!!editing?.candidate}
					placeholder="Next available"
					bind:value={number}
				/>
			</div>
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label for="c-first">First name</Label>
					<Input id="c-first" required bind:value={firstName} />
				</div>
				<div class="space-y-2">
					<Label for="c-last">Last name</Label>
					<Input id="c-last" required bind:value={lastName} />
				</div>
			</div>
			{#if save.isError}
				<p class="text-sm text-destructive" role="alert">{save.error.message}</p>
			{/if}
			<Dialog.Footer>
				<Button type="button" variant="outline" onclick={() => (editing = null)}>Cancel</Button>
				<Button type="submit" disabled={save.isPending}>Save</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<ConfirmDialog
	bind:open={() => deleting !== null, (v) => !v && (deleting = null)}
	title={`Delete candidate ${shownDeleting.current?.number} (${shownDeleting.current?.first_name} ${shownDeleting.current?.last_name})?`}
	description="Candidates who already have scores cannot be deleted."
	confirmLabel="Delete"
	onconfirm={() => {
		if (deleting) {
			remove.mutate(deleting);
		}
		deleting = null;
	}}
/>

<ConfirmDialog
	bind:open={renumbering}
	title="Renumber candidates?"
	description="Numbers become 1, 2, 3... in the current order, closing any gaps."
	confirmLabel="Renumber"
	onconfirm={() => {
		renumber.mutate();
		renumbering = false;
	}}
/>

<ConfirmDialog
	bind:open={bulkDeleting}
	title={`Delete ${selected.length} candidates?`}
	description="Candidates who already have scores cannot be deleted and will be skipped."
	confirmLabel="Delete"
	onconfirm={() => {
		removeMany.mutate(selected);
		bulkDeleting = false;
	}}
/>

<ImportDialog bind:open={importing} />
