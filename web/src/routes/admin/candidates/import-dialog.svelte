<script lang="ts">
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import ImportForm from "./import-form.svelte";

	let { open = $bindable() }: { open: boolean } = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-w-2xl">
		<ImportForm onclose={() => (open = false)} />
	</Dialog.Content>
</Dialog.Root>
