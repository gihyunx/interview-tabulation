<script lang="ts">
	import { page } from "$app/state";
	import { ApiError, api, errorMessage } from "#lib/api/client.ts";
	import ConfirmDialog from "#lib/components/confirm-dialog.svelte";
	import StateMessage from "#lib/components/state-message.svelte";
	import { Badge } from "#lib/components/ui/badge/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Progress } from "#lib/components/ui/progress/index.js";
	import * as Sheet from "#lib/components/ui/sheet/index.js";
	import { Slider } from "#lib/components/ui/slider/index.js";
	import { clearDraft, loadDraft, saveDraft } from "#lib/draft.ts";
	import { boardQuery, myInterviewsQuery } from "#lib/queries.ts";
	import { clampScore, totalOf } from "#lib/scoring.ts";
	import { createMutation, createQuery, useQueryClient } from "@tanstack/svelte-query";
	import { toast } from "svelte-sonner";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import UsersIcon from "@lucide/svelte/icons/users";
	import CandidatePicker from "./candidate-picker.svelte";

	const interviewId = $derived(page.params.interviewId!);
	const queryClient = useQueryClient();

	const mine = createQuery(myInterviewsQuery);
	const board = createQuery(() => boardQuery(interviewId));

	const interview = $derived(mine.data?.find((i) => i.id === interviewId));
	const criteria = $derived(interview?.rubric.criteria ?? []);
	const candidates = $derived(board.data?.candidates ?? []);
	const judgeId = $derived(board.data?.judge_id ?? "");

	const scoredCount = $derived(candidates.filter((c) => c.status === "scored").length);
	const chosen = $derived(candidates.find((c) => c.id === selectedId));
	// A 1-on-1 candidate another judge has taken cannot be scored, so it is not "selected".
	const selected = $derived(chosen && chosen.status !== "scored_by_other" ? chosen : null);

	// What the server has for the selected candidate.
	const saved = $derived<Record<string, number>>(
		Object.fromEntries((selected?.my_scores ?? []).map((s) => [s.criterion_id, s.value]))
	);

	let selectedId = $state<string | null>(null);
	let values = $state<Record<string, number | undefined>>({});
	let restored = $state(false);
	let pendingId = $state<string | null>(null); // candidate we are asking to switch to
	let reviewing = $state(false); // the submit confirm dialog
	let picking = $state(false); // the candidate drawer on small screens
	let recenter = $state(0); // bumped when a switch is cancelled, so the list scrolls back
	let attempted = $state(false); // submit was pressed, so show what is missing

	const entered = $derived(criteria.filter((c) => values[c.id] !== undefined).length);
	const total = $derived(totalOf(criteria.map((c) => values[c.id])));
	const dirty = $derived(
		selected !== null &&
			criteria.some((c) => values[c.id] !== saved[c.id]) &&
			criteria.some((c) => values[c.id] !== undefined)
	);
	const isUpdate = $derived(selected?.status === "scored");

	// Submit stays enabled; pressing it explains what is in the way instead.
	const missing = $derived(criteria.filter((c) => values[c.id] === undefined));
	const problem = $derived(
		missing.length > 0
			? `Score ${missing.length === 1 ? "this criterion" : "these criteria"} first: ${missing.map((c) => c.title).join(", ")}.`
			: isUpdate && !dirty
				? "Nothing has changed since these scores were submitted."
				: null
	);

	function trySubmit() {
		attempted = true;
		if (problem === null) {
			reviewing = true;
		} else if (missing.length > 0) {
			// Scroll only: focusing would pop up the on-screen keyboard on a tablet.
			document
				.getElementById(`criterion-${missing[0].id}`)
				?.scrollIntoView({ block: "center", behavior: "smooth" });
		}
	}

	// The rubric scrolls on its own and its footer sticks over the bottom, so say when criteria are
	// hidden under it. Checked on scroll, resize, and when the criteria change height.
	let moreBelow = $state(false);
	let criteriaList: HTMLElement | undefined;
	let footer: HTMLElement | undefined;
	function checkMore() {
		if (criteriaList && footer) {
			moreBelow =
				criteriaList.getBoundingClientRect().bottom > footer.getBoundingClientRect().top + 1;
		}
	}
	function track(set: (el: HTMLElement) => void) {
		return (el: HTMLElement) => {
			set(el);
			const observer = new ResizeObserver(checkMore);
			observer.observe(el);
			return () => observer.disconnect();
		};
	}
	const trackCriteria = track((el) => (criteriaList = el));
	const trackFooter = track((el) => (footer = el));
	// Bring the first criterion that is still under the footer into view.
	function showMore() {
		const top = footer?.getBoundingClientRect().top ?? 0;
		const next = [...(criteriaList?.children ?? [])].find(
			(row) => row.getBoundingClientRect().bottom > top
		);
		next?.scrollIntoView({ block: "center", behavior: "smooth" });
	}

	const lastKey = $derived(`last:${judgeId}:${interviewId}`);

	// Load the form for a candidate: draft first, else what is saved.
	function open(id: string) {
		selectedId = id;
		attempted = false;
		const c = candidates.find((x) => x.id === id);
		const draft = loadDraft(judgeId, interviewId, id);
		restored = draft !== null;
		values =
			draft ?? Object.fromEntries((c?.my_scores ?? []).map((s) => [s.criterion_id, s.value]));
		try {
			localStorage.setItem(lastKey, id);
		} catch {
			// Not remembering the last candidate is fine.
		}
	}

	function requestSelect(id: string) {
		if (id === selectedId) {
			return;
		}
		if (dirty) {
			pendingId = id;
		} else {
			open(id);
		}
	}

	// Back to the last candidate after a reload, once the board is here.
	let resumed = false;
	$effect(() => {
		if (resumed || !judgeId || candidates.length === 0) {
			return;
		}
		resumed = true;
		try {
			const last = localStorage.getItem(lastKey);
			if (last && candidates.some((c) => c.id === last && c.status !== "scored_by_other")) {
				open(last);
			}
		} catch {
			// No storage, no resume.
		}
	});

	// Autosave unsubmitted inputs; nothing to keep once they match the server.
	$effect(() => {
		const id = selected?.id;
		if (!id || !judgeId) {
			return;
		}
		const draft: Record<string, number> = {};
		for (const c of criteria) {
			const v = values[c.id];
			if (v !== undefined) {
				draft[c.id] = v;
			}
		}
		if (dirty) {
			saveDraft(judgeId, interviewId, id, draft);
		} else {
			clearDraft(judgeId, interviewId, id);
		}
	});

	// Tell the judge when another judge takes the candidate they had open.
	let warnedFor: string | null = null;
	$effect(() => {
		if (chosen?.status === "scored_by_other" && warnedFor !== chosen.id) {
			warnedFor = chosen.id;
			toast.error(`Candidate ${chosen.number} was scored by ${chosen.scored_by}.`);
		}
	});

	function setValue(id: string, max: number, v: number) {
		values[id] = clampScore(v, max);
		restored = false;
	}

	function cancel() {
		if (!selectedId) {
			return;
		}
		clearDraft(judgeId, interviewId, selectedId);
		values = { ...saved };
		restored = false;
	}

	const submit = createMutation(() => ({
		mutationFn: async (candidateId: string) => {
			const { error, response } = await api.PUT(
				"/v1/interviews/{id}/candidates/{candidateId}/scores",
				{
					params: { path: { id: interviewId, candidateId } },
					body: { scores: criteria.map((c) => ({ criterion_id: c.id, value: values[c.id] ?? 0 })) }
				}
			);
			if (error) {
				throw new ApiError(errorMessage(error, "Could not save the scores."), response.status);
			}
			return candidateId;
		},
		// Retrying is safe (the PUT is idempotent), but only for network and server errors.
		retry: (count: number, error: Error) =>
			count < 3 && !(error instanceof ApiError && error.status < 500),
		onSuccess: async (candidateId) => {
			clearDraft(judgeId, interviewId, candidateId);
			attempted = false;
			toast.success(isUpdate ? "Scores updated." : "Scores submitted.");
			// Stay on this candidate; the judge picks the next one.
			await queryClient.invalidateQueries({ queryKey: ["board", interviewId] });
		},
		onError: (e) => {
			toast.error(e.message); // the draft stays
			void queryClient.invalidateQueries({ queryKey: ["board", interviewId] });
		}
	}));

	const modeLabel = $derived(interview?.mode === "panel" ? "Panel interview" : "1-on-1 interview");
</script>

<!-- Below lg the page scrolls instead of the rubric card. -->
<svelte:window onscroll={checkMore} onresize={checkMore} />

<svelte:head><title>{interview?.title ?? "Scoring"}</title></svelte:head>

<StateMessage
	pending={mine.isPending || board.isPending}
	error={mine.error ?? board.error}
	empty={mine.data && !interview ? "You are not assigned to this interview." : undefined}
/>

{#if interview && board.data}
	<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
		<div class="flex flex-wrap items-center gap-3">
			<h1 class="text-3xl font-bold">{interview.title}</h1>
			<Badge variant="secondary">{modeLabel}</Badge>
		</div>
		<div class="w-72">
			<p class="mb-2 text-right text-sm text-muted-foreground" role="status">
				<span class="font-bold text-foreground">{scoredCount}</span> of {candidates.length} scored
			</p>
			<Progress value={candidates.length ? (scoredCount / candidates.length) * 100 : 0} />
		</div>
	</div>

	<!-- On large screens both columns fill the rest of the viewport and scroll on their own. -->
	<div class="grid gap-6 lg:min-h-0 lg:flex-1 lg:grid-cols-5">
		<section
			class="rounded-xl border bg-card p-6 shadow-xs lg:col-span-3 lg:overflow-y-auto lg:p-8"
			aria-label="Rubric"
			onscroll={checkMore}
		>
			<div class="flex items-center justify-between gap-4">
				<h2 class="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
					{interview.rubric.name}
				</h2>
				{#if selected}
					<Button variant="outline" class="lg:hidden" onclick={() => (picking = true)}>
						<UsersIcon />
						Change candidate
					</Button>
				{/if}
			</div>
			{#if !selected}
				<div class="flex flex-col items-center gap-4 py-16">
					<p class="text-lg text-muted-foreground">Select a candidate to start scoring.</p>
					<Button size="lg" class="lg:hidden" onclick={() => (picking = true)}>
						<UsersIcon />
						Choose candidate
					</Button>
				</div>
			{:else}
				<p class="mt-1 mb-6 text-2xl tracking-tight text-muted-foreground">
					Scoring
					<span class="font-bold text-foreground">candidate #{selected.number}</span>
					<span class="text-lg">({selected.last_name}, {selected.first_name})</span>
				</p>
				{#if restored}
					<p class="mb-4 rounded-md bg-warning p-2 text-sm text-warning-foreground" role="status">
						Draft restored. These scores are not submitted yet.
					</p>
				{/if}

				<div class="divide-y" {@attach trackCriteria}>
					{#each criteria as c, n (c.id)}
						<div id={`criterion-${c.id}`} class="py-5 first:pt-0">
							<div class="flex items-start justify-between gap-4">
								<div>
									<p class="text-lg font-semibold">
										<span class="font-medium text-muted-foreground">{n + 1}.</span>
										{c.title}
									</p>
									{#if c.description}<p class="mt-1 text-muted-foreground">
											{c.description}
										</p>{/if}
								</div>
								<div class="flex shrink-0 items-center gap-2">
									<Input
										type="number"
										inputmode="numeric"
										min={0}
										max={c.max_points}
										placeholder="--"
										class="no-spinner h-12 w-20 border-2 border-muted-foreground/50 text-center text-xl font-bold shadow-sm placeholder:text-muted-foreground/60 md:text-xl"
										aria-label={`${c.title} score`}
										aria-invalid={attempted && values[c.id] === undefined}
										value={values[c.id] ?? ""}
										oninput={(e) => {
											const raw = e.currentTarget.value;
											if (raw === "") {
												values[c.id] = undefined;
											} else {
												setValue(c.id, c.max_points, Number(raw));
											}
										}}
										onblur={(e) => (e.currentTarget.value = String(values[c.id] ?? ""))}
									/>
									<span class="text-muted-foreground">/ {c.max_points}</span>
								</div>
							</div>
							<Slider
								type="single"
								class="mt-4 py-3"
								min={0}
								max={c.max_points}
								step={1}
								value={values[c.id] ?? 0}
								onValueChange={(v) => setValue(c.id, c.max_points, v)}
								aria-label={`${c.title} slider`}
							/>
							{#if attempted && values[c.id] === undefined}
								<p class="text-sm font-medium text-destructive">Enter a score.</p>
							{/if}
						</div>
					{/each}
				</div>

				<!-- Stays in view at the bottom of the rubric card. Sticky stops at the padding edge, so the
				     negative bottom offset and margins match the card padding to sit flush with its border. -->
				<div
					class="sticky -bottom-6 -mx-6 mt-4 -mb-6 bg-card px-6 pb-6 lg:-bottom-8 lg:-mx-8 lg:-mb-8 lg:px-8 lg:pb-8"
					{@attach trackFooter}
				>
					{#if moreBelow}
						<div
							class="pointer-events-none absolute inset-x-0 bottom-full h-16 bg-linear-to-t from-card"
						></div>
						<Button
							variant="secondary"
							class="absolute bottom-full left-1/2 mb-3 -translate-x-1/2 rounded-full shadow-md"
							onclick={showMore}
						>
							<ChevronDownIcon />
							More criteria below
						</Button>
					{/if}
					<div class="flex flex-wrap items-center justify-between gap-4 border-t pt-4">
						<div>
							<p class="text-sm text-muted-foreground" role="status">
								{entered} of {criteria.length} criteria scored
							</p>
							<p class="text-4xl font-extrabold tracking-tight">
								{total} <span class="text-xl font-medium text-muted-foreground">/ 100</span>
							</p>
						</div>
						<div class="flex gap-2">
							<Button variant="outline" size="lg" disabled={!dirty} onclick={cancel}>Cancel</Button>
							<Button size="lg" disabled={submit.isPending} onclick={trySubmit}>
								{submit.isPending ? "Saving..." : isUpdate ? "Update scores" : "Submit scores"}
							</Button>
						</div>
					</div>
					{#if attempted && problem}
						<p class="mt-3 text-sm font-medium text-destructive" role="alert">{problem}</p>
					{/if}
				</div>
			{/if}
		</section>

		<!-- Below lg the list lives in a drawer, so it is not buried under the rubric. -->
		<section class="hidden min-h-0 lg:col-span-2 lg:block" aria-label="Candidates">
			<CandidatePicker
				{candidates}
				selectedId={selected?.id ?? null}
				onselect={requestSelect}
				{recenter}
			/>
		</section>
	</div>
{/if}

<Sheet.Root bind:open={picking}>
	<!-- No autofocus: focusing the search box would pop up the on-screen keyboard on every open. -->
	<Sheet.Content
		class="w-full data-[side=right]:w-full data-[side=right]:sm:max-w-md"
		onOpenAutoFocus={(e) => e.preventDefault()}
	>
		<!-- The base sheet caps the right side at max-w-sm with a data-side variant; only the same variant overrides it. -->
		<Sheet.Header>
			<Sheet.Title class="text-xl">Candidates</Sheet.Title>
		</Sheet.Header>
		<div class="min-h-0 flex-1 px-4 pb-4">
			<CandidatePicker
				{candidates}
				selectedId={selected?.id ?? null}
				onselect={requestSelect}
				onchoose={() => (picking = false)}
				{recenter}
			/>
		</div>
	</Sheet.Content>
</Sheet.Root>

<ConfirmDialog
	bind:open={reviewing}
	title={`${isUpdate ? "Update" : "Submit"} scores for candidate ${selected?.number} (${selected?.last_name}, ${selected?.first_name})?`}
	description="Check the scores below. You can edit them later."
	confirmLabel={isUpdate ? "Update" : "Submit"}
	onconfirm={() => selectedId && submit.mutate(selectedId)}
	large
>
	<ul class="space-y-2 text-lg">
		{#each criteria as c (c.id)}
			<li class="flex justify-between gap-4">
				<span>{c.title}</span>
				<span class="shrink-0"
					><span class="font-bold">{values[c.id] ?? 0}</span>
					<span class="text-muted-foreground">/ {c.max_points}</span></span
				>
			</li>
		{/each}
		<li class="flex justify-between border-t pt-3 text-xl font-bold">
			<span>Total</span><span>{total} / 100</span>
		</li>
	</ul>
</ConfirmDialog>

<ConfirmDialog
	bind:open={
		() => pendingId !== null,
		(v) => {
			if (!v) {
				pendingId = null;
				recenter++;
			}
		}
	}
	title="Switch candidate?"
	description="You have scores that are not submitted. They are kept as a draft, and you can come back to them."
	confirmLabel="Switch candidate"
	onconfirm={() => {
		if (pendingId) {
			open(pendingId);
		}
		pendingId = null;
	}}
/>
