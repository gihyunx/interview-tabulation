<script lang="ts">
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
	import ConnectionPill from "#lib/components/connection-pill.svelte";
	import LogoutButton from "#lib/components/logout-button.svelte";
	import ChartColumnIcon from "@lucide/svelte/icons/chart-column";
	import ClipboardListIcon from "@lucide/svelte/icons/clipboard-list";
	import MicIcon from "@lucide/svelte/icons/mic";
	import UserRoundIcon from "@lucide/svelte/icons/user-round";
	import UsersIcon from "@lucide/svelte/icons/users";
	import type { LayoutProps } from "./$types";

	let { children }: LayoutProps = $props();

	const nav = [
		{ href: "/admin/scores", label: "Scores", icon: ChartColumnIcon },
		{ href: "/admin/rubrics", label: "Rubrics", icon: ClipboardListIcon },
		{ href: "/admin/interviews", label: "Interviews", icon: MicIcon },
		{ href: "/admin/candidates", label: "Candidates", icon: UserRoundIcon },
		{ href: "/admin/users", label: "Users", icon: UsersIcon }
	] as const;
</script>

<div class="flex min-h-screen">
	<aside class="fixed inset-y-0 left-0 flex w-60 flex-col border-r bg-sidebar p-4">
		<div class="mb-8 px-3 pt-2">
			<p class="text-lg font-extrabold tracking-tight text-sidebar-primary">Interview Tabulation</p>
			<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">Admin</p>
		</div>
		<nav class="flex-1 space-y-1">
			{#each nav as item (item.href)}
				{@const current = page.url.pathname.startsWith(item.href)}
				<a
					href={resolve(item.href)}
					aria-current={current ? "page" : undefined}
					class={[
						"flex items-center gap-3 rounded-lg px-3 py-2 font-medium text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
						current && "bg-sidebar-accent font-semibold text-sidebar-accent-foreground"
					]}
				>
					<item.icon class="size-5" aria-hidden="true" />
					{item.label}
				</a>
			{/each}
		</nav>
		<LogoutButton class="w-full" />
	</aside>
	<main class="ml-60 min-w-0 flex-1 p-8">
		<div class="mb-4 flex justify-end"><ConnectionPill /></div>
		{@render children()}
	</main>
</div>
