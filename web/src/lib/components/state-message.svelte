<script lang="ts">
	// Loading / error / empty text for a TanStack query, so every screen shows the same states.
	type Props = { pending: boolean; error: Error | null; empty?: string };
	let { pending, error, empty }: Props = $props();
</script>

{#if pending}
	<p class="text-muted-foreground" role="status">Loading...</p>
{:else if error}
	<p class="text-destructive" role="alert">{error.message}</p>
{:else if empty}
	<p class="text-muted-foreground">{empty}</p>
{/if}
