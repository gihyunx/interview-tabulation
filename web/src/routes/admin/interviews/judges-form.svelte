<script lang="ts">
	import { untrack } from "svelte";
	import { api, errorMessage } from "#lib/api/client.ts";
	import type { components } from "#lib/api/schema.d.ts";
	import { Button } from "#lib/components/ui/button/index.js";
	import { Checkbox } from "#lib/components/ui/checkbox/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import { usersQuery } from "#lib/queries.ts";
	import { createMutation, createQuery, useQueryClient } from "@tanstack/svelte-query";
	import { toast } from "svelte-sonner";

	type Interview = components["schemas"]["Interview"];

	let { interview, onclose }: { interview: Interview; onclose: () => void } = $props();

	const queryClient = useQueryClient();
	const users = createQuery(usersQuery);
	const judges = $derived((users.data ?? []).filter((u) => u.role === "judge"));

	// The dialog mounts this fresh each time it opens, so the list starts from the props.
	let selected = $state(untrack(() => (interview.judges ?? []).map((j) => j.user_id)));

	// A judge who already has scores cannot be unassigned.
	const scored = $derived(
		new Set((interview.judges ?? []).filter((j) => j.scored > 0).map((j) => j.user_id))
	);

	function toggle(id: string, on: boolean) {
		selected = on ? [...selected, id] : selected.filter((x) => x !== id);
	}

	const save = createMutation(() => ({
		mutationFn: async () => {
			const { error } = await api.PUT("/v1/interviews/{id}/judges", {
				params: { path: { id: interview.id } },
				body: { user_ids: selected }
			});
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => {
			toast.success("Judges saved.");
			onclose();
			void queryClient.invalidateQueries({ queryKey: ["interviews"] });
		}
	}));
</script>

<Dialog.Header>
	<Dialog.Title>Judges for {interview.title}</Dialog.Title>
	<Dialog.Description>
		Only users with the judge role are listed. Judges who already have scores stay assigned.
	</Dialog.Description>
</Dialog.Header>

<ul class="max-h-72 space-y-2 overflow-y-auto">
	{#each judges as u (u.id)}
		<li>
			<label class="flex items-center gap-3 text-sm">
				<Checkbox
					checked={selected.includes(u.id)}
					disabled={scored.has(u.id)}
					onCheckedChange={(on) => toggle(u.id, on === true)}
				/>
				{u.last_name}, {u.first_name}
				<span class="text-muted-foreground">({u.username})</span>
			</label>
		</li>
	{:else}
		<li class="text-sm text-muted-foreground">No judge users yet. Add some on the Users page.</li>
	{/each}
</ul>

{#if save.isError}
	<p class="text-sm text-destructive" role="alert">{save.error.message}</p>
{/if}

<Dialog.Footer>
	<Button variant="outline" onclick={() => onclose()}>Cancel</Button>
	<Button disabled={save.isPending} onclick={() => save.mutate()}>Save</Button>
</Dialog.Footer>
