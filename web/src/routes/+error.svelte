<script lang="ts">
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
</script>

<svelte:head><title>Something went wrong</title></svelte:head>

<main class="flex min-h-screen flex-col items-center justify-center gap-4 p-8 text-center">
	<h1 class="text-3xl font-bold">
		{page.status === 404 ? "Page not found" : "Something went wrong"}
	</h1>
	<p class="text-muted-foreground">{page.error?.message}</p>
	<a href={resolve("/")} class="text-primary underline">Back to start</a>
</main>
