<script lang="ts">
	import { resolve } from "$app/paths";
	import type { LiveShow } from "@/types";
	import LivePoster from "./LivePoster.svelte";
	import { formatShowDate } from "./date";

	interface Props {
		show: LiveShow;
	}

	let { show }: Props = $props();
</script>

<a class="live-card" href={resolve(`/live/${show.id}`)} data-state={show.state}>
	<span class="poster"><LivePoster poster={show.poster} /></span>
	<span class="text">
		<span class="date">{formatShowDate(show.date)}</span>
		<span class="title">{show.title}</span>
		{#if show.venue || show.state === "missed"}
			<span class="meta">
				{#if show.venue}<span class="venue">{show.venue}</span>{/if}
				{#if show.state === "missed"}<span class="missed">Didn't go</span>{/if}
			</span>
		{/if}
	</span>
</a>

<style lang="scss">
	.live-card {
		display: flex;
		flex-flow: row;
		align-items: flex-start;
		gap: 12px;
		padding: 10px 14px 10px 10px;
		border: 1px solid $bg-color-accent;
		border-radius: 8px;
		color: $text-color;
		transition: border-color 150ms ease;

		// Always there (placeholder included), so titles line up down the list.
		.poster {
			display: block;
			flex: 0 0 56px;
			width: 56px;
		}

		.text {
			display: flex;
			flex-flow: column;
			gap: 3px;
			min-width: 0;
		}

		.date {
			font-size: 13px;
			color: $text-color-accent;
			font-variant-numeric: tabular-nums;
		}

		.title {
			font-size: 16px;
			font-weight: bold;
			overflow-wrap: anywhere;
		}

		.meta {
			display: flex;
			flex-flow: row wrap;
			align-items: center;
			gap: 4px 10px;
			font-size: 14px;
			color: $text-color-accent;
		}

		// Discreet on purpose: a show you skipped is still part of the history,
		// it shouldn't shout from the list.
		.missed {
			font-size: 12px;
			padding: 1px 7px;
			border: 1px solid $bg-color-accent;
			border-radius: 10px;
		}

		&[data-state="missed"] {
			.title,
			.poster {
				opacity: 0.7;
			}
		}

		@media (hover: hover) {
			&:hover {
				border-color: $text-color;
			}
		}

		&:focus-visible {
			border-color: $text-color;
		}
	}
</style>
