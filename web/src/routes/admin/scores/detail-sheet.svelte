<script lang="ts">
	import { api, errorMessage } from "#lib/api/client.ts";
	import type { components } from "#lib/api/schema.d.ts";
	import ConfirmDialog from "#lib/components/confirm-dialog.svelte";
	import { Button } from "#lib/components/ui/button/index.js";
	import * as Sheet from "#lib/components/ui/sheet/index.js";
	import * as Table from "#lib/components/ui/table/index.js";
	import { useQueryClient, createMutation } from "@tanstack/svelte-query";
	import { toast } from "svelte-sonner";

	type Candidate = components["schemas"]["ResultCandidate"];
	type Interview = components["schemas"]["ResultInterview"];
	type Submission = components["schemas"]["Submission"];

	type Props = { candidate: Candidate | null; selected: Interview[]; onclose: () => void };
	let { candidate, selected, onclose }: Props = $props();

	const queryClient = useQueryClient();
	let clearing = $state<Submission | null>(null);

	const clear = createMutation(() => ({
		mutationFn: async (s: Submission) => {
			const { error } = await api.DELETE("/v1/interviews/{id}/candidates/{candidateId}/scores", {
				params: {
					path: { id: s.interview_id, candidateId: candidate!.id },
					query: { judge: s.judge_id }
				}
			});
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => toast.success("Submission cleared."),
		onError: (e) => toast.error(e.message),
		onSettled: () => queryClient.invalidateQueries({ queryKey: ["results"] })
	}));

	const subsOf = (interviewId: string) =>
		(candidate?.submissions ?? []).filter((s) => s.interview_id === interviewId);
	const valueOf = (s: Submission, criterionId: string) =>
		(s.values ?? []).find((v) => v.criterion_id === criterionId)?.value;
</script>

<Sheet.Root bind:open={() => candidate !== null, (v) => !v && onclose()}>
	<!-- The base sheet caps the right side at max-w-sm with a data-side variant; only the same variant overrides it. -->
	<Sheet.Content
		class="w-full overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-3xl"
	>
		<Sheet.Header>
			<Sheet.Title class="text-2xl font-bold tracking-tight"
				>Candidate {candidate?.number}
				<span class="block text-base font-normal text-muted-foreground"
					>{candidate?.last_name}, {candidate?.first_name}</span
				></Sheet.Title
			>
			<Sheet.Description>
				{#if candidate?.complete && candidate.average != null}
					Average {candidate.average.toFixed(2)} / 100
				{:else}
					Average pending until every score is in.
				{/if}
			</Sheet.Description>
		</Sheet.Header>

		<div class="space-y-6 px-4 pb-6">
			{#each selected as i (i.id)}
				{@const subs = subsOf(i.id)}
				<section>
					<h3 class="mb-2 font-bold">{i.title}</h3>
					{#if subs.length === 0}
						<p class="text-sm text-muted-foreground">Not scored yet.</p>
					{:else}
						<Table.Root>
							<Table.Header>
								<Table.Row>
									<Table.Head>Criterion</Table.Head>
									{#each subs as s (s.judge_id)}
										<Table.Head class="text-center">{s.judge_name}</Table.Head>
									{/each}
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each i.criteria ?? [] as c (c.id)}
									<Table.Row>
										<Table.Cell>{c.title}</Table.Cell>
										{#each subs as s (s.judge_id)}
											<Table.Cell class="text-center"
												>{valueOf(s, c.id) ?? "-"} / {c.max_points}</Table.Cell
											>
										{/each}
									</Table.Row>
								{/each}
								<Table.Row class="font-bold">
									<Table.Cell>Total</Table.Cell>
									{#each subs as s (s.judge_id)}
										<Table.Cell class="text-center">{s.total}</Table.Cell>
									{/each}
								</Table.Row>
								<Table.Row>
									<Table.Cell></Table.Cell>
									{#each subs as s (s.judge_id)}
										<Table.Cell class="text-center">
											<Button variant="outline" size="sm" onclick={() => (clearing = s)}>
												Clear submission
											</Button>
										</Table.Cell>
									{/each}
								</Table.Row>
							</Table.Body>
						</Table.Root>
					{/if}
				</section>
			{/each}
		</div>
	</Sheet.Content>
</Sheet.Root>

<ConfirmDialog
	bind:open={() => clearing !== null, (v) => !v && (clearing = null)}
	title={`Clear ${clearing?.judge_name}'s scores?`}
	description={`This deletes their scores for candidate ${candidate?.number} (${candidate?.first_name} ${candidate?.last_name}). In a 1-on-1 interview, other judges can then score this candidate. It cannot be undone.`}
	confirmLabel="Clear scores"
	onconfirm={() => {
		if (clearing) {
			clear.mutate(clearing);
		}
		clearing = null;
	}}
/>
