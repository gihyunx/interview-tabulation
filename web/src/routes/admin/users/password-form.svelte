<script lang="ts">
	import { api, errorMessage } from "#lib/api/client.ts";
	import type { User } from "#lib/session.ts";
	import { Button } from "#lib/components/ui/button/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Label } from "#lib/components/ui/label/index.js";
	import { createMutation } from "@tanstack/svelte-query";

	type Props = {
		user: User;
		onclose: () => void;
		onsaved: (message: string) => void;
	};
	let { user, onclose, onsaved }: Props = $props();

	let password = $state("");

	const reset = createMutation(() => ({
		mutationFn: async () => {
			const { error } = await api.POST("/v1/users/{id}/password", {
				params: { path: { id: user.id } },
				body: { password }
			});
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => {
			onsaved("Password reset.");
			onclose();
		}
	}));
</script>

<form
	class="space-y-4"
	onsubmit={(e) => {
		e.preventDefault();
		reset.mutate();
	}}
>
	<Dialog.Header>
		<Dialog.Title>Reset password</Dialog.Title>
		<Dialog.Description>
			Set a new password for {user.first_name}
			{user.last_name}. Tell them in person.
		</Dialog.Description>
	</Dialog.Header>

	<div class="space-y-2">
		<Label for="reset-password">New password</Label>
		<Input
			id="reset-password"
			type="password"
			autocomplete="new-password"
			required
			minlength={8}
			maxlength={72}
			bind:value={password}
		/>
	</div>

	{#if reset.isError}
		<p class="text-sm text-destructive" role="alert">{reset.error.message}</p>
	{/if}

	<Dialog.Footer>
		<Button type="button" variant="outline" onclick={onclose}>Cancel</Button>
		<Button type="submit" disabled={reset.isPending}>Reset password</Button>
	</Dialog.Footer>
</form>
