<script lang="ts">
	import { goto, invalidateAll } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { api, errorMessage } from "#lib/api/client.ts";
	import { homeFor } from "#lib/session.ts";
	import { Button } from "#lib/components/ui/button/index.js";
	import PasswordInput from "#lib/components/password-input.svelte";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Label } from "#lib/components/ui/label/index.js";
	import { createMutation } from "@tanstack/svelte-query";

	let username = $state("");
	let password = $state("");

	const login = createMutation(() => ({
		mutationFn: async () => {
			const { data, error } = await api.POST("/v1/auth/login", {
				body: { username, password }
			});
			if (error) {
				throw new Error(errorMessage(error, "Could not log in. Try again."));
			}
			return data;
		},
		onSuccess: async (user) => {
			await invalidateAll();
			await goto(resolve(homeFor(user)));
		}
	}));
</script>

<svelte:head><title>Log in</title></svelte:head>

<main class="flex min-h-screen items-center justify-center bg-muted p-4">
	<form
		class="w-full max-w-sm space-y-6 rounded-xl border bg-card p-8 shadow-sm"
		onsubmit={(e) => {
			e.preventDefault();
			login.mutate();
		}}
	>
		<div class="space-y-1">
			<h1 class="text-3xl font-extrabold tracking-tight text-primary">Interview Tabulation</h1>
			<p class="text-sm text-muted-foreground">Log in to continue.</p>
		</div>

		<div class="space-y-2">
			<Label for="username">Username</Label>
			<Input
				id="username"
				autocomplete="username"
				autocapitalize="none"
				required
				bind:value={username}
			/>
		</div>
		<div class="space-y-2">
			<Label for="password">Password</Label>
			<PasswordInput id="password" autocomplete="current-password" required bind:value={password} />
		</div>

		{#if login.isError}
			<p class="text-sm text-destructive" role="alert">{login.error.message}</p>
		{/if}

		<Button type="submit" class="w-full" disabled={login.isPending}>
			{login.isPending ? "Logging in..." : "Log in"}
		</Button>
	</form>
</main>
