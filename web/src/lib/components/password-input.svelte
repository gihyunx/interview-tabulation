<script lang="ts">
	import { Input } from "#lib/components/ui/input/index.js";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import type { HTMLInputAttributes } from "svelte/elements";

	let {
		value = $bindable(),
		class: className,
		...props
	}: Omit<HTMLInputAttributes, "type" | "files"> = $props();

	let shown = $state(false);
</script>

<div class="relative">
	<Input type={shown ? "text" : "password"} class={["pr-9", className]} bind:value {...props} />
	<button
		type="button"
		class="absolute inset-y-0 right-0 flex w-9 items-center justify-center text-muted-foreground hover:text-foreground"
		aria-label={shown ? "Hide password" : "Show password"}
		aria-pressed={shown}
		onclick={() => (shown = !shown)}
	>
		{#if shown}
			<EyeOffIcon class="size-4" aria-hidden="true" />
		{:else}
			<EyeIcon class="size-4" aria-hidden="true" />
		{/if}
	</button>
</div>
