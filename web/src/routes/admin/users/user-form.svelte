<script lang="ts">
	import { untrack } from "svelte";
	import { api, errorMessage } from "#lib/api/client.ts";
	import type { User } from "#lib/session.ts";
	import { Button } from "#lib/components/ui/button/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Label } from "#lib/components/ui/label/index.js";
	import * as Select from "#lib/components/ui/select/index.js";
	import { createMutation } from "@tanstack/svelte-query";

	type Props = {
		/** The user being edited; none = create. */
		user?: User;
		/** Editing your own account: the role cannot change. */
		isSelf: boolean;
		onclose: () => void;
		onsaved: (message: string) => void;
	};
	let { user, isSelf, onclose, onsaved }: Props = $props();

	// The dialog mounts this fresh each time it opens, so the form starts from the props.
	const start = untrack(() => user);
	let username = $state(start?.username ?? "");
	let password = $state("");
	let firstName = $state(start?.first_name ?? "");
	let lastName = $state(start?.last_name ?? "");
	let role = $state<User["role"]>(start?.role ?? "judge");

	const save = createMutation(() => ({
		mutationFn: async () => {
			const { error } = user
				? await api.PATCH("/v1/users/{id}", {
						params: { path: { id: user.id } },
						// Role is left out for yourself: the server refuses to change it.
						body: {
							username,
							first_name: firstName,
							last_name: lastName,
							...(isSelf ? {} : { role })
						}
					})
				: await api.POST("/v1/users", {
						body: { username, password, role, first_name: firstName, last_name: lastName }
					});
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => {
			onsaved(user ? "User updated." : "User created.");
			onclose();
		}
	}));
</script>

<form
	class="space-y-4"
	onsubmit={(e) => {
		e.preventDefault();
		save.mutate();
	}}
>
	<Dialog.Header>
		<Dialog.Title>{user ? "Edit user" : "New user"}</Dialog.Title>
		<Dialog.Description>
			{user ? "Change this user's details." : "Create a judge or an admin account."}
		</Dialog.Description>
	</Dialog.Header>

	<div class="grid grid-cols-2 gap-4">
		<div class="space-y-2">
			<Label for="first-name">First name</Label>
			<Input id="first-name" required bind:value={firstName} />
		</div>
		<div class="space-y-2">
			<Label for="last-name">Last name</Label>
			<Input id="last-name" required bind:value={lastName} />
		</div>
	</div>
	<div class="space-y-2">
		<Label for="new-username">Username</Label>
		<Input
			id="new-username"
			autocapitalize="none"
			required
			minlength={3}
			pattern="[A-Za-z0-9._\-]+"
			title="Letters, numbers, dot, dash and underscore."
			bind:value={username}
		/>
	</div>
	{#if !user}
		<div class="space-y-2">
			<Label for="new-password">Password</Label>
			<Input
				id="new-password"
				type="password"
				autocomplete="new-password"
				required
				minlength={8}
				maxlength={72}
				bind:value={password}
			/>
		</div>
	{/if}
	<div class="space-y-2">
		<Label for="role">Role</Label>
		<Select.Root type="single" bind:value={role} disabled={isSelf}>
			<Select.Trigger id="role" class="w-full">
				{role === "admin" ? "Admin" : "Judge"}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value="judge" label="Judge" />
				<Select.Item value="admin" label="Admin" />
			</Select.Content>
		</Select.Root>
	</div>

	{#if save.isError}
		<p class="text-sm text-destructive" role="alert">{save.error.message}</p>
	{/if}

	<Dialog.Footer>
		<Button type="button" variant="outline" onclick={onclose}>Cancel</Button>
		<Button type="submit" disabled={save.isPending}>Save</Button>
	</Dialog.Footer>
</form>
