<script lang="ts">
	import type { components } from "#lib/api/schema.d.ts";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import JudgesForm from "./judges-form.svelte";

	type Interview = components["schemas"]["Interview"];

	let { interview = $bindable() }: { interview: Interview | null } = $props();
</script>

<Dialog.Root bind:open={() => interview !== null, (v) => !v && (interview = null)}>
	<Dialog.Content>
		{#if interview}
			<JudgesForm {interview} onclose={() => (interview = null)} />
		{/if}
	</Dialog.Content>
</Dialog.Root>
