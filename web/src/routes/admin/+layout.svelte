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

<div class="min-h-screen md:flex">
	<aside
		class="fixed inset-x-0 bottom-0 z-10 flex items-center gap-2 border-t bg-sidebar p-2 md:inset-y-0 md:right-auto md:w-60 md:flex-col md:items-stretch md:border-t-0 md:border-r md:p-4"
	>
		<div class="hidden px-3 pt-2 md:mb-8 md:block">
			<p class="text-lg font-extrabold tracking-tight text-sidebar-primary">Interview Tabulation</p>
			<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">Admin</p>
		</div>
		<nav class="flex min-w-0 flex-1 gap-1 overflow-x-auto md:flex-col">
			{#each nav as item (item.href)}
				{@const current = page.url.pathname.startsWith(item.href)}
				<a
					href={resolve(item.href)}
					aria-current={current ? "page" : undefined}
					class={[
						"flex shrink-0 items-center gap-3 rounded-lg px-3 py-2 font-medium text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
						current && "bg-sidebar-accent font-semibold text-sidebar-accent-foreground"
					]}
				>
					<item.icon class="size-5" aria-hidden="true" />
					<span class="sr-only sm:not-sr-only">{item.label}</span>
				</a>
			{/each}
		</nav>
		<LogoutButton class="md:w-full" />
	</aside>
	<main class="min-w-0 flex-1 p-4 pb-20 md:ml-60 md:p-8">
		<div class="mb-4 flex justify-end"><ConnectionPill /></div>
		{@render children()}
	</main>
</div>
