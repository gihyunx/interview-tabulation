<script lang="ts">
	import { api, errorMessage } from "#lib/api/client.ts";
	import { latest } from "#lib/latest.svelte.ts";
	import type { User } from "#lib/session.ts";
	import * as AlertDialog from "#lib/components/ui/alert-dialog/index.js";
	import { createMutation } from "@tanstack/svelte-query";
	import { toast } from "svelte-sonner";

	type Props = {
		user: User | null;
		onclose: () => void;
		onsaved: (message: string) => void;
	};
	let { user, onclose, onsaved }: Props = $props();
	const shown = latest(() => user);

	const remove = createMutation(() => ({
		mutationFn: async () => {
			if (!user) {
				return;
			}
			const { error } = await api.DELETE("/v1/users/{id}", {
				params: { path: { id: user.id } }
			});
			if (error) {
				throw new Error(errorMessage(error));
			}
		},
		onSuccess: () => onsaved("User deleted."),
		// The server explains why (e.g. "already has scores").
		onError: (error) => toast.error(error.message),
		onSettled: onclose
	}));
</script>

<AlertDialog.Root open={user !== null} onOpenChange={(open) => !open && onclose()}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title
				>Delete {shown.current?.first_name} {shown.current?.last_name}?</AlertDialog.Title
			>
			<AlertDialog.Description>
				This also removes their interview assignments. Users who already have scores cannot be
				deleted.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action onclick={() => remove.mutate()}>Delete</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
