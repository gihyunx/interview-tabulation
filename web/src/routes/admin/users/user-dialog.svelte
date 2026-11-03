<script lang="ts">
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import type { User } from "#lib/session.ts";
	import UserForm from "./user-form.svelte";

	type Props = {
		/** null = closed; `{}` = create; `{ user }` = edit. */
		state: { user?: User } | null;
		isSelf: boolean;
		onclose: () => void;
		onsaved: (message: string) => void;
	};
	let { state: target, isSelf, onclose, onsaved }: Props = $props();
</script>

<Dialog.Root open={target !== null} onOpenChange={(open) => !open && onclose()}>
	<Dialog.Content>
		{#if target}
			<UserForm user={target.user} {isSelf} {onclose} {onsaved} />
		{/if}
	</Dialog.Content>
</Dialog.Root>
