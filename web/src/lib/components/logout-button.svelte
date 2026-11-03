<script lang="ts">
	import { goto, invalidateAll } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { api } from "#lib/api/client.ts";
	import { Button, type ButtonProps } from "#lib/components/ui/button/index.js";
	import { useQueryClient } from "@tanstack/svelte-query";

	let { ...props }: ButtonProps = $props();

	const queryClient = useQueryClient();

	async function logout() {
		await api.POST("/v1/auth/logout");
		queryClient.clear();
		await invalidateAll();
		await goto(resolve("/login"));
	}
</script>

<Button variant="outline" onclick={logout} {...props}>Log out</Button>
