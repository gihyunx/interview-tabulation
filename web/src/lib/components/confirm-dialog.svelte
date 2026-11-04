<script lang="ts">
	import * as AlertDialog from "#lib/components/ui/alert-dialog/index.js";
	import type { Snippet } from "svelte";

	type Props = {
		open: boolean;
		title: string;
		description?: string;
		confirmLabel: string;
		onconfirm: () => void;
		children?: Snippet;
		// Wider, with bigger text, for dialogs that are read carefully (e.g. reviewing scores).
		large?: boolean;
	};
	let {
		open = $bindable(),
		title,
		description,
		confirmLabel,
		onconfirm,
		children,
		large = false
	}: Props = $props();
</script>

<AlertDialog.Root bind:open>
	<!-- The base content sets its width with a data-size variant; only the same variant overrides it. -->
	<AlertDialog.Content class={large ? "data-[size=default]:sm:max-w-xl" : undefined}>
		<AlertDialog.Header>
			<AlertDialog.Title class={large ? "text-2xl font-bold tracking-tight" : undefined}
				>{title}</AlertDialog.Title
			>
			{#if description}
				<AlertDialog.Description class={large ? "text-base md:text-base" : undefined}
					>{description}</AlertDialog.Description
				>
			{/if}
		</AlertDialog.Header>
		{@render children?.()}
		<AlertDialog.Footer>
			<AlertDialog.Cancel size={large ? "lg" : "default"}>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action
				size={large ? "lg" : "default"}
				onclick={() => {
					onconfirm();
					open = false;
				}}>{confirmLabel}</AlertDialog.Action
			>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
