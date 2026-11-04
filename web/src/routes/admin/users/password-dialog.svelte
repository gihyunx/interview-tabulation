<script lang="ts">
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import type { User } from "#lib/session.ts";
	import PasswordForm from "./password-form.svelte";

	type Props = {
		user: User | null;
		onclose: () => void;
		onsaved: (message: string) => void;
	};
	let { user, onclose, onsaved }: Props = $props();
</script>

<Dialog.Root open={user !== null} onOpenChange={(open) => !open && onclose()}>
	<Dialog.Content>
		{#if user}
			<PasswordForm {user} {onclose} {onsaved} />
		{/if}
	</Dialog.Content>
</Dialog.Root>
