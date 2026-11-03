<script lang="ts">
	import { untrack } from "svelte";
	import { api, errorMessage } from "#lib/api/client.ts";
	import type { components } from "#lib/api/schema.d.ts";
	import { Button } from "#lib/components/ui/button/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Label } from "#lib/components/ui/label/index.js";
	import * as Select from "#lib/components/ui/select/index.js";
	import { Textarea } from "#lib/components/ui/textarea/index.js";
	import { rubricsQuery } from "#lib/queries.ts";
	import { createMutation, createQuery, useQueryClient } from "@tanstack/svelte-query";
	import { toast } from "svelte-sonner";

	type Interview = components["schemas"]["Interview"];
	type Mode = Interview["mode"];

	/** The interview being edited; none = create. */
	let { interview, onclose }: { interview?: Interview; onclose: () => void } = $props();

	const queryClient = useQueryClient();
	const rubrics = createQuery(rubricsQuery);

	// The dialog mounts this fresh each time it opens, so the form starts from the props.
	const start = untrack(() => interview);
	let title = $state(start?.title ?? "");
	let description = $state(start?.description ?? "");
	let mode = $state<Mode>(start?.mode ?? "one_on_one");
	// A new interview starts on the first rubric once the list is here.
	let rubricId = $derived(start?.rubric_id ?? rubrics.data?.[0]?.id ?? "");

	// A scored interview keeps its rubric and mode.
	const frozen = $derived(interview?.has_scores ?? false);

	const save = createMutation(() => ({
		mutationFn: async () => {
			const body = { title, description, mode, rubric_id: rubricId };
			const existing = interview;
			const { error } = existing
				? await api.PATCH("/v1/interviews/{id}", { params: { path: { id: existing.id } }, body })
				: await api.POST("/v1/interviews", { body });
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => {
			toast.success(interview ? "Interview updated." : "Interview created.");
			onclose();
			void queryClient.invalidateQueries({ queryKey: ["interviews"] });
		}
	}));

	const modes = [
		{ value: "one_on_one", label: "1-on-1" },
		{ value: "panel", label: "Panel" }
	] as const;
</script>

<form
	class="space-y-4"
	onsubmit={(e) => {
		e.preventDefault();
		save.mutate();
	}}
>
	<Dialog.Header>
		<Dialog.Title>{interview ? "Edit interview" : "New interview"}</Dialog.Title>
		<Dialog.Description>
			{frozen
				? "This interview has scores, so its rubric and mode are fixed."
				: "A stage of the process, with one rubric."}
		</Dialog.Description>
	</Dialog.Header>

	<div class="space-y-2">
		<Label for="iv-title">Title</Label>
		<Input id="iv-title" required bind:value={title} />
	</div>
	<div class="space-y-2">
		<Label for="iv-desc">Description</Label>
		<Textarea id="iv-desc" rows={2} bind:value={description} />
	</div>
	<div class="grid grid-cols-2 gap-4">
		<div class="space-y-2">
			<Label for="iv-mode">Mode</Label>
			<Select.Root type="single" bind:value={mode} disabled={frozen}>
				<Select.Trigger id="iv-mode" class="w-full">
					{modes.find((m) => m.value === mode)?.label}
				</Select.Trigger>
				<Select.Content>
					{#each modes as m (m.value)}
						<Select.Item value={m.value} label={m.label} />
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
		<div class="space-y-2">
			<Label for="iv-rubric">Rubric</Label>
			<Select.Root type="single" bind:value={rubricId} disabled={frozen}>
				<Select.Trigger id="iv-rubric" class="w-full">
					{rubrics.data?.find((r) => r.id === rubricId)?.name ?? "Choose a rubric"}
				</Select.Trigger>
				<Select.Content>
					{#each rubrics.data ?? [] as r (r.id)}
						<Select.Item value={r.id} label={r.name} />
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
	</div>

	{#if save.isError}
		<p class="text-sm text-destructive" role="alert">{save.error.message}</p>
	{/if}

	<Dialog.Footer>
		<Button type="button" variant="outline" onclick={() => onclose()}>Cancel</Button>
		<Button type="submit" disabled={save.isPending || !rubricId}>Save</Button>
	</Dialog.Footer>
</form>
