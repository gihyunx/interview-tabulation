<script lang="ts">
	import { api, errorMessage } from "#lib/api/client.ts";
	import ConfirmDialog from "#lib/components/confirm-dialog.svelte";
	import StateMessage from "#lib/components/state-message.svelte";
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
	let number = $state<number | null>(null);
	let firstName = $state("");
	let lastName = $state("");
	let deleting = $state<Candidate | null>(null);
	let importing = $state(false);

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

<div class="mb-6 flex items-center justify-between gap-4">
	<h1 class="text-3xl font-bold">Candidates</h1>
	<div class="flex gap-2">
		<Button variant="outline" onclick={() => (importing = true)}>Import CSV</Button>
		<Button onclick={() => openForm()}>New candidate</Button>
	</div>
</div>

<Input
	class="mb-4 w-80"
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
					<Table.Head class="w-24">No.</Table.Head>
					<Table.Head>Name</Table.Head>
					<Table.Head class="text-right">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each shown as c (c.id)}
					<Table.Row>
						<Table.Cell class="text-lg font-bold tabular-nums">{c.number}</Table.Cell>
						<Table.Cell class="text-muted-foreground">{c.last_name}, {c.first_name}</Table.Cell>
						<Table.Cell class="space-x-2 text-right">
							<Button variant="outline" size="sm" onclick={() => openForm(c)}>Edit</Button>
							<Button variant="outline" size="sm" onclick={() => (deleting = c)}>Delete</Button>
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row>
						<Table.Cell colspan={3} class="text-center text-muted-foreground">
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
				<Dialog.Title>{editing?.candidate ? "Edit candidate" : "New candidate"}</Dialog.Title>
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
	title={`Delete candidate ${deleting?.number} (${deleting?.first_name} ${deleting?.last_name})?`}
	description="Candidates who already have scores cannot be deleted."
	confirmLabel="Delete"
	onconfirm={() => {
		if (deleting) {
			remove.mutate(deleting);
		}
		deleting = null;
	}}
/>

<ImportDialog bind:open={importing} />
