<script lang="ts">
	import { resolve } from "$app/paths";
	import Icon from "@/lib/Icon.svelte";
	import type { TmdbPersonCandidate } from "@/types";

	interface Props {
		c: TmdbPersonCandidate;
	}

	let { c }: Props = $props();

	// "Acting · Holy Motors (2012), Mauvais Sang (1986)": what tells two
	// namesakes apart, in one line.
	let details = $derived.by(() => {
		const known = c.knownFor
			.map((k) => (k.year ? `${k.title} (${k.year})` : k.title))
			.join(", ");
		return [c.department, known].filter(Boolean).join(" · ");
	});
</script>

<!-- The content of one suggestion; the option itself (and picking) belongs to
     the list around it. -->
<span class="photo">
	{#if c.profilePath}
		<img
			src={`https://image.tmdb.org/t/p/w185${c.profilePath}`}
			alt=""
			loading="lazy"
		/>
	{:else}
		<Icon i="person" wh={18} />
	{/if}
</span>
<span class="text">
	<span class="name-line">
		<span class="name">{c.name}</span>
		{#if c.inYourList > 0}
			<span class="in-list">{c.inYourList} in your list</span>
		{/if}
	</span>
	{#if details}
		<span class="details" title={details}>{details}</span>
	{/if}
</span>
<!-- Checking isn't choosing: the click stops here, and the link stays out of
     the Tab order (the list closes when the input loses focus). -->
<a
	class="check"
	href={resolve(`/person/${c.tmdbId}`)}
	target="_blank"
	rel="noopener"
	tabindex="-1"
	title="Open their page in a new tab"
	aria-label="Open their page in a new tab"
	onclick={(e) => e.stopPropagation()}
>
	<Icon i="eye" wh={18} />
</a>

<style lang="scss">
	.photo {
		display: flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 32px;
		width: 32px;
		height: 48px;
		overflow: hidden;
		border-radius: 4px;
		background-color: $bg-color-accent;
		fill: $text-color-accent;

		img {
			width: 100%;
			height: 100%;
			object-fit: cover;
		}
	}

	.text {
		display: flex;
		flex-flow: column;
		gap: 2px;
		flex: 1 1 auto;
		min-width: 0;
	}

	.name-line {
		display: flex;
		flex-flow: row wrap;
		align-items: center;
		gap: 2px 8px;
		min-width: 0;
	}

	.name {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	// The strongest hint that it's the right person: it has to stand out.
	.in-list {
		flex: 0 0 auto;
		padding: 1px 7px;
		border-radius: 10px;
		font-size: 11px;
		font-weight: bold;
		white-space: nowrap;
		color: white;
		background-color: $success;
	}

	.details {
		overflow: hidden;
		font-size: 12px;
		white-space: nowrap;
		text-overflow: ellipsis;
		color: $text-color-accent;
	}

	.check {
		display: flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 36px;
		height: 36px;
		border-radius: 6px;
		fill: $text-color-accent;

		@media (hover: hover) {
			&:hover {
				fill: $text-color;
				outline: 1px solid $text-color-accent;
			}
		}
	}
</style>
