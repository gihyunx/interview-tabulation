<script lang="ts">
	import { Slider as SliderPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "#lib/utils.js";

	let {
		ref = $bindable(null),
		value = $bindable(),
		orientation = "horizontal",
		class: className,
		...restProps
	}: WithoutChildrenOrChild<SliderPrimitive.RootProps> = $props();
</script>

<!--
Discriminated Unions + Destructing (required for bindable) do not
get along, so we shut typescript up by casting `value` to `never`.
-->
<!--
On touch screens only the thumb takes input: the track lets swipes through so scrolling the page
past a slider does not change its value. The thumb's touch area is grown to about 48px.
-->
<SliderPrimitive.Root
	bind:ref
	bind:value={value as never}
	data-slot="slider"
	{orientation}
	class={cn(
		"relative flex w-full touch-none items-center select-none pointer-coarse:pointer-events-none data-disabled:opacity-50 data-vertical:h-full data-vertical:min-h-40 data-vertical:w-auto data-vertical:flex-col",
		className
	)}
	{...restProps}
>
	{#snippet children({ thumbItems })}
		<span
			data-slot="slider-track"
			data-orientation={orientation}
			class={cn(
				"relative grow overflow-hidden rounded-full bg-secondary data-horizontal:h-2.5 data-horizontal:w-full data-vertical:h-full data-vertical:h-full data-vertical:w-2.5"
			)}
		>
			<SliderPrimitive.Range
				data-slot="slider-range"
				class={cn("absolute bg-primary select-none data-horizontal:h-full data-vertical:w-full")}
			/>
		</span>
		{#each thumbItems as thumb (thumb.index)}
			<SliderPrimitive.Thumb
				data-slot="slider-thumb"
				index={thumb.index}
				class="block size-7 shrink-0 cursor-grab rounded-full border-2 border-primary bg-white shadow-md ring-ring/50 transition-[color,box-shadow] select-none after:absolute after:-inset-2.5 after:rounded-full hover:ring-4 focus-visible:ring-4 focus-visible:outline-hidden active:cursor-grabbing disabled:pointer-events-none disabled:opacity-50 pointer-coarse:pointer-events-auto"
			/>
		{/each}
	{/snippet}
</SliderPrimitive.Root>
