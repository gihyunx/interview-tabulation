<script lang="ts">
	import type { components } from "#lib/api/schema.d.ts";
	import { Input } from "#lib/components/ui/input/index.js";
	import { matchScore } from "#lib/search.ts";
	import CheckIcon from "@lucide/svelte/icons/check";
	import LockIcon from "@lucide/svelte/icons/lock";

	type Candidate = components["schemas"]["BoardCandidate"];

	type Props = {
		candidates: Candidate[];
		selectedId: string | null;
		onselect: (id: string) => void;
		// Called after a row is tapped or clicked (not on search, scroll or arrow keys), e.g. to close a drawer.
		onchoose?: () => void;
		// Change it to scroll the selected row back to the middle, e.g. after a switch was cancelled.
		recenter?: number;
	};
	let { candidates, selectedId, onselect, onchoose, recenter }: Props = $props();

	let search = $state("");

	const scored = (c: Candidate) =>
		c.status === "scored_by_other"
			? `Scored by ${c.scored_by}`
			: c.status === "scored"
				? `Scored ${c.total}/100`
				: "Not yet scored";
	const selectable = (c: Candidate) => c.status !== "scored_by_other";

	const matches = $derived(
		candidates
			.map((c) => ({ c, score: matchScore(search, `${c.first_name} ${c.last_name}`) }))
			.filter((m) => m.score > 0)
	);
	// A-Z by last name is the server's order. The list shows matches only.
	const shown = $derived(matches.map((m) => m.c));
	const selectedIndex = $derived(shown.findIndex((c) => c.id === selectedId));

	// Typing jumps to the best selectable match.
	function onSearch() {
		if (!search.trim()) {
			return;
		}
		const best = matches.filter((m) => selectable(m.c)).sort((a, b) => b.score - a.score)[0];
		if (best && best.c.id !== selectedId) {
			onselect(best.c.id);
		}
	}

	function move(step: 1 | -1) {
		let i = selectedIndex;
		do {
			i += step;
		} while (i >= 0 && i < shown.length && !selectable(shown[i]));
		if (i >= 0 && i < shown.length) {
			onselect(shown[i].id);
		}
	}

	function onkeydown(e: KeyboardEvent) {
		if (e.key === "ArrowDown") {
			move(1);
		} else if (e.key === "ArrowUp") {
			move(-1);
		} else {
			userScrolled = true; // Page Down, Home, Space and the like scroll the list natively
			return;
		}
		e.preventDefault();
	}

	// Half a list of empty space above the first row and below the last, so any row can scroll to the
	// middle and the list scrolls even with a few candidates. Kept in step with the list's height.
	function pad(list: HTMLElement) {
		list.style.paddingBlock = `${list.clientHeight / 2}px`;
	}
	function keepPadded(list: HTMLElement) {
		pad(list);
		const observer = new ResizeObserver(() => {
			pad(list);
			arc(list);
		});
		observer.observe(list);
		return () => observer.disconnect();
	}

	// Scroll the selected row to the middle. Re-runs when the selection or the list changes.
	function centerSelected(list: HTMLElement) {
		void selectedId;
		void shown;
		void recenter;
		// A selection (click, keys, search, or a settled scroll) wins over any scroll still in flight:
		// drop the pending settle and stop following the scroll until the judge scrolls again.
		clearTimeout(settleTimer);
		userScrolled = false;
		skipMomentum = true;
		scrollToSelected(list);
		arc(list);
	}
	function scrollToSelected(list: HTMLElement) {
		const row = list.querySelector<HTMLElement>("[aria-selected=true]");
		if (row) {
			const top = row.offsetTop + row.offsetHeight / 2 - list.clientHeight / 2;
			if (Math.abs(list.scrollTop - top) > 1) {
				list.scrollTo({ top, behavior: "smooth" });
			}
		}
	}

	// Song-wheel look: rows ease aside and fade with their distance from the middle of the list,
	// measured on screen, so the curve follows free scrolling too.
	function arc(list: HTMLElement) {
		const mid = list.scrollTop + list.clientHeight / 2;
		for (const row of list.children as HTMLCollectionOf<HTMLElement>) {
			const d = Math.abs(row.offsetTop + row.offsetHeight / 2 - mid) / (list.clientHeight / 2);
			row.style.setProperty("--d", Math.min(d, 1).toFixed(3));
		}
	}
	// Song-wheel selection: the middle of the list is the selection. Scrolling snaps to the nearest
	// selectable row, and once it stops, that row is selected. Only after the judge scrolls: the
	// browser's own snap when the list first shows must not pick a candidate for them.
	let settleTimer: ReturnType<typeof setTimeout> | undefined;
	let userScrolled = false;
	const markUserScroll = () => (userScrolled = true);

	// A trackpad keeps sending wheel events while it coasts, also after a click. Those must not scroll
	// away from what was just picked. Coasting events come every frame; a new gesture starts after a
	// pause, and only that counts as the judge scrolling again.
	let skipMomentum = false;
	let lastWheel = 0;
	function guardWheel(list: HTMLElement) {
		const onwheel = (e: WheelEvent) => {
			const newGesture = e.timeStamp - lastWheel > 100;
			lastWheel = e.timeStamp;
			if (skipMomentum && !newGesture) {
				e.preventDefault();
				return;
			}
			skipMomentum = false;
			markUserScroll();
		};
		// Not passive, so coasting events can be cancelled.
		list.addEventListener("wheel", onwheel, { passive: false });
		return () => list.removeEventListener("wheel", onwheel);
	}
	// Dragging the scrollbar scrolls; pressing a row is a click, not a scroll.
	const onpointerdown = (e: PointerEvent) => e.target === e.currentTarget && markUserScroll();
	function onscroll(e: Event & { currentTarget: HTMLElement }) {
		const list = e.currentTarget;
		arc(list);
		clearTimeout(settleTimer);
		settleTimer = setTimeout(() => settle(list), 150);
	}
	function settle(list: HTMLElement) {
		if (!userScrolled) {
			// Not the judge's scroll (e.g. a wheel animation that outran a click): go back to the selection.
			scrollToSelected(list);
			return;
		}
		const mid = list.scrollTop + list.clientHeight / 2;
		let best: HTMLElement | undefined;
		let bestD = Infinity;
		for (const row of list.querySelectorAll<HTMLElement>("[role=option]:not(:disabled)")) {
			const d = Math.abs(row.offsetTop + row.offsetHeight / 2 - mid);
			if (d < bestD) {
				best = row;
				bestD = d;
			}
		}
		const id = best?.dataset.id;
		if (id && id !== selectedId) {
			onselect(id);
		}
	}

	// Indent from the left so the right edge stays put and nothing is clipped.
	const arcStyle = "width: calc(100% - var(--d, 0) * 3rem); opacity: calc(1 - var(--d, 0) * 0.4)";

	const locked = $derived(candidates.filter((c) => !selectable(c)).length);
</script>

<div class="flex h-full flex-col gap-3">
	<Input
		type="search"
		placeholder="Search candidates"
		aria-label="Search candidates"
		class="h-11"
		bind:value={search}
		oninput={onSearch}
	/>
	<div class="flex justify-between text-sm text-muted-foreground">
		<span aria-live="polite">
			{shown.length === candidates.length
				? `${candidates.length} candidates`
				: `${shown.length} of ${candidates.length} match`}
		</span>
		<span>A - Z</span>
	</div>
	{#if locked > 0}
		<p
			class="flex items-start gap-2 rounded-lg bg-secondary px-3 py-2 text-sm text-secondary-foreground"
		>
			<LockIcon class="mt-0.5 size-4 shrink-0" />
			<span>
				{locked === 1 ? "1 candidate is" : `${locked} candidates are`} locked: another judge already scored
				them, and in a 1-on-1 interview each candidate has one interviewer.
			</span>
		</p>
	{/if}

	<div
		{@attach keepPadded}
		{@attach guardWheel}
		{@attach centerSelected}
		{onscroll}
		ontouchstart={markUserScroll}
		{onpointerdown}
		role="listbox"
		aria-label="Candidates"
		tabindex="0"
		{onkeydown}
		class="relative min-h-0 flex-1 snap-y snap-mandatory space-y-2 overflow-x-hidden overflow-y-auto rounded-lg focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
	>
		{#each shown as c (c.id)}
			{@const active = c.id === selectedId}
			<button
				type="button"
				role="option"
				data-id={c.id}
				aria-selected={active}
				disabled={!selectable(c)}
				title={selectable(c) ? undefined : `Locked: ${scored(c)}`}
				tabindex="-1"
				onclick={() => {
					onselect(c.id);
					onchoose?.();
				}}
				style={arcStyle}
				class={[
					// Every row is the same height, so selecting one never shifts the others.
					"ml-auto flex w-full items-center justify-between gap-3 rounded-xl border px-5 py-4 text-left transition-colors",
					selectable(c) && "snap-center",
					active
						? "border-primary bg-primary text-primary-foreground shadow-lg"
						: "bg-card shadow-xs hover:border-ring",
					"disabled:cursor-not-allowed disabled:border-dashed disabled:bg-muted disabled:shadow-none disabled:hover:border-border"
				]}
			>
				<span class="min-w-0">
					<span class="block truncate text-lg font-bold">{c.last_name}, {c.first_name}</span>
					<span
						class={[
							"block text-sm",
							active ? "text-primary-foreground/80" : "text-muted-foreground"
						]}
					>
						{scored(c)}
					</span>
				</span>
				{#if !selectable(c)}
					<LockIcon class="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
				{:else if c.status === "scored"}
					<CheckIcon
						class={["size-5 shrink-0", active ? "text-primary-foreground" : "text-primary"]}
						aria-hidden="true"
					/>
				{/if}
			</button>
		{:else}
			<p class="py-8 text-center text-muted-foreground">
				{candidates.length === 0 ? "No candidates yet." : "No candidates match."}
			</p>
		{/each}
	</div>
</div>
