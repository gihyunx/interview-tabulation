<script lang="ts">
	import { usersQuery } from "#lib/queries.ts";
	import type { User } from "#lib/session.ts";
	import { Badge } from "#lib/components/ui/badge/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import * as Table from "#lib/components/ui/table/index.js";
	import { createQuery, useQueryClient } from "@tanstack/svelte-query";
	import { toast } from "svelte-sonner";
	import type { PageProps } from "./$types";
	import DeleteUserDialog from "./delete-user-dialog.svelte";
	import PasswordDialog from "./password-dialog.svelte";
	import UserDialog from "./user-dialog.svelte";

	let { data }: PageProps = $props();

	const queryClient = useQueryClient();

	const users = createQuery(usersQuery);

	// null = closed. { user: undefined } on the form dialog = create.
	let editing = $state<{ user?: User } | null>(null);
	let resetting = $state<User | null>(null);
	let deleting = $state<User | null>(null);

	function saved(message: string) {
		toast.success(message);
		void queryClient.invalidateQueries({ queryKey: ["users"] });
	}
</script>

<svelte:head><title>Users</title></svelte:head>

<div class="mb-6 flex items-center justify-between">
	<h1 class="text-3xl font-bold">Users</h1>
	<Button onclick={() => (editing = {})}>New user</Button>
</div>

{#if users.isPending}
	<p class="text-muted-foreground">Loading users...</p>
{:else if users.isError}
	<p class="text-destructive" role="alert">{users.error.message}</p>
{:else}
	<div class="overflow-hidden rounded-xl border bg-card shadow-xs">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>Name</Table.Head>
					<Table.Head>Username</Table.Head>
					<Table.Head>Role</Table.Head>
					<Table.Head class="text-right">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each users.data as user (user.id)}
					<Table.Row>
						<Table.Cell class="font-semibold">{user.last_name}, {user.first_name}</Table.Cell>
						<Table.Cell>{user.username}</Table.Cell>
						<Table.Cell>
							<Badge variant={user.role === "admin" ? "default" : "secondary"}>{user.role}</Badge>
						</Table.Cell>
						<Table.Cell class="space-x-2 text-right">
							<Button variant="outline" size="sm" onclick={() => (editing = { user })}>Edit</Button>
							<Button variant="outline" size="sm" onclick={() => (resetting = user)}>
								Reset password
							</Button>
							<Button
								variant="outline"
								size="sm"
								disabled={user.id === data.user?.id}
								onclick={() => (deleting = user)}
							>
								Delete
							</Button>
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row>
						<Table.Cell colspan={4} class="text-center text-muted-foreground">
							No users yet.
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}

<UserDialog
	state={editing}
	isSelf={editing?.user?.id === data.user?.id}
	onclose={() => (editing = null)}
	onsaved={saved}
/>
<PasswordDialog user={resetting} onclose={() => (resetting = null)} onsaved={saved} />
<DeleteUserDialog user={deleting} onclose={() => (deleting = null)} onsaved={saved} />
