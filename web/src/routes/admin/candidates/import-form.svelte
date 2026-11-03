<script lang="ts">
	import { api, errorMessage } from "#lib/api/client.ts";
	import { Button } from "#lib/components/ui/button/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Label } from "#lib/components/ui/label/index.js";
	import * as Table from "#lib/components/ui/table/index.js";
	import { createMutation } from "@tanstack/svelte-query";
	import { toast } from "svelte-sonner";

	let { onclose }: { onclose: () => void } = $props();

	let file = $state<File | null>(null);

	// The server parses the file. First with dry_run for the preview, then for real.
	async function send(dryRun: boolean) {
		const f = file;
		if (!f) {
			throw new Error("Choose a CSV file first.");
		}
		const { data, error } = await api.POST("/v1/candidates/import", {
			params: { query: { dry_run: dryRun } },
			// The generated type models the file as a string; the serializer sends real form data.
			body: { file: f as unknown as string },
			bodySerializer: () => {
				const fd = new FormData();
				fd.append("file", f);
				return fd;
			}
		});
		if (error) {
			throw new Error(errorMessage(error, "Could not read the file."));
		}
		return data;
	}

	const preview = createMutation(() => ({ mutationFn: () => send(true) }));
	const confirm = createMutation(() => ({
		mutationFn: () => send(false),
		onSuccess: (data) => {
			toast.success(`Imported ${data.rows?.length ?? 0} candidates.`);
			onclose();
		}
	}));

	const rows = $derived(preview.data?.rows ?? []);
	const canImport = $derived(preview.isSuccess && rows.length > 0 && preview.data?.errors === 0);
</script>

<Dialog.Header>
	<Dialog.Title>Import candidates</Dialog.Title>
	<Dialog.Description>
		Upload a CSV with the header <code>first_name,last_name</code>. You can review it before
		anything is saved.
	</Dialog.Description>
</Dialog.Header>

<form
	class="flex items-end gap-2"
	onsubmit={(e) => {
		e.preventDefault();
		confirm.reset();
		preview.mutate();
	}}
>
	<div class="flex-1 space-y-2">
		<Label for="csv">CSV file</Label>
		<Input
			id="csv"
			type="file"
			accept=".csv,text/csv"
			required
			onchange={(e) => {
				file = e.currentTarget.files?.[0] ?? null;
				preview.reset();
			}}
		/>
	</div>
	<Button type="submit" variant="outline" disabled={!file || preview.isPending}>Preview</Button>
</form>

{#if preview.isError}
	<p class="text-sm text-destructive" role="alert">{preview.error.message}</p>
{/if}

{#if preview.isSuccess}
	<p class="text-sm" role="status">
		{rows.length} rows, {preview.data.errors} with errors.
	</p>
	<div class="max-h-64 overflow-y-auto rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>Line</Table.Head>
					<Table.Head>First name</Table.Head>
					<Table.Head>Last name</Table.Head>
					<Table.Head>Problem</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each rows as row (row.line)}
					<Table.Row>
						<Table.Cell>{row.line}</Table.Cell>
						<Table.Cell>{row.first_name}</Table.Cell>
						<Table.Cell>{row.last_name}</Table.Cell>
						<Table.Cell class="text-destructive">{row.error ?? ""}</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}

{#if confirm.isError}
	<p class="text-sm text-destructive" role="alert">{confirm.error.message}</p>
{/if}

<Dialog.Footer>
	<Button variant="outline" onclick={onclose}>Cancel</Button>
	<Button disabled={!canImport || confirm.isPending} onclick={() => confirm.mutate()}>
		Import {canImport ? rows.length : ""} candidates
	</Button>
</Dialog.Footer>
