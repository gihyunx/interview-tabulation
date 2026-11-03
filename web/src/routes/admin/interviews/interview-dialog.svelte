<script lang="ts">
	import type { components } from "#lib/api/schema.d.ts";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import InterviewForm from "./interview-form.svelte";

	type Interview = components["schemas"]["Interview"];

	/** null = closed; `{}` = create; `{ interview }` = edit. */
	let { state: target = $bindable() }: { state: { interview?: Interview } | null } = $props();
</script>

<Dialog.Root bind:open={() => target !== null, (v) => !v && (target = null)}>
	<Dialog.Content>
		{#if target}
			<InterviewForm interview={target.interview} onclose={() => (target = null)} />
		{/if}
	</Dialog.Content>
</Dialog.Root>
