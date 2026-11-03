<script lang="ts">
	import { connection } from "#lib/realtime.svelte.ts";
	import { Badge } from "#lib/components/ui/badge/index.js";
</script>

<Badge variant={connection.live ? "secondary" : "destructive"} role="status">
	{connection.live ? "Live" : "Reconnecting..."}
</Badge>
