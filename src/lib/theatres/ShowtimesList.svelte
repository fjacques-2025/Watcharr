<script lang="ts">
	import { resolve } from "$app/paths";
	import Icon from "@/lib/Icon.svelte";
	import type { TheatreFilm } from "@/types";

	interface Props {
		films: TheatreFilm[];
	}

	let { films }: Props = $props();

	function runtimeLabel(mins?: number) {
		if (!mins) return "";
		const h = Math.floor(mins / 60);
		return h > 0 ? `${h}h${String(mins % 60).padStart(2, "0")}` : `${mins}min`;
	}
</script>

<!-- A day's programme is a schedule, not a wall of posters: one row per film,
     showtimes as the payload. -->
<ul class="showtimes">
	{#each films as f (f.title)}
		{@const link = f.media?.ids?.tmdb
			? resolve(`/movie/${f.media.ids.tmdb}`)
			: undefined}
		<li>
			<div class="poster">
				{#if link}
					<a href={link}>
						{#if f.media?.extPosterPath}
							<img
								src={`https://image.tmdb.org/t/p/w185${f.media.extPosterPath}`}
								alt={f.title}
								loading="lazy"
							/>
						{:else}
							<div class="noposter"><Icon i="reel" wh={28} /></div>
						{/if}
					</a>
				{:else}
					<div class="noposter"><Icon i="reel" wh={28} /></div>
				{/if}
			</div>

			<div class="body">
				<h3>
					{#if link}
						<a href={link}>{f.title}</a>
					{:else}
						{f.title}
					{/if}
				</h3>
				<div class="meta">
					{#if f.genre}<span>{f.genre}</span>{/if}
					{#if f.runtime}<span>{runtimeLabel(f.runtime)}</span>{/if}
					{#if !f.media}
						<!-- Say so rather than quietly showing a bare row: it's usually a
						     retrospective or a one-off, not a bug. -->
						<span
							class="unmatched"
							title="Not found on TMDB, so no poster or details — often the case for retrospectives and one-off screenings."
						>
							not on TMDB
						</span>
					{/if}
				</div>

				{#each f.screenings as sc (sc.theatre)}
					<div class="venue">
						<span class="name">{sc.theatre}</span>
						<div class="slots">
							{#each sc.showtimes as st (st.start + (st.bookingUrl ?? ""))}
								{@const label = st.end
									? `${st.start} – ends ${st.end}`
									: st.start}
								{#if st.bookingUrl}
									<a
										class="slot"
										href={st.bookingUrl}
										target="_blank"
										rel="noreferrer noopener"
										title={`Book ${f.title} at ${sc.theatre}, ${label}`}
									>
										{st.start}
										{#if st.version}<em>{st.version}</em>{/if}
									</a>
								{:else}
									<span class="slot" title={label}>
										{st.start}
										{#if st.version}<em>{st.version}</em>{/if}
									</span>
								{/if}
							{/each}
						</div>
					</div>
				{/each}
			</div>
		</li>
	{/each}
</ul>

<style lang="scss">
	.showtimes {
		display: flex;
		flex-flow: column;
		gap: 14px;
		list-style: none;
		padding: 0 15px;
		margin: 0;

		li {
			display: flex;
			flex-flow: row;
			gap: 14px;
			padding: 12px;
			border-radius: 10px;
			background-color: $accent-color;
		}
	}

	.poster {
		flex: 0 0 auto;

		img,
		.noposter {
			width: 78px;
			height: 117px;
			border-radius: 6px;
			object-fit: cover;
			display: block;
		}

		.noposter {
			display: flex;
			align-items: center;
			justify-content: center;
			background-color: $bg-color-accent;
			fill: $text-color;
			opacity: 0.6;
		}
	}

	.body {
		display: flex;
		flex-flow: column;
		gap: 6px;
		min-width: 0;
		flex: 1 1 auto;

		h3 {
			font-size: 17px;
			margin: 0;

			a {
				color: $text-color;
				text-decoration: none;

				&:hover,
				&:focus-visible {
					text-decoration: underline;
				}
			}
		}
	}

	.meta {
		display: flex;
		flex-flow: row wrap;
		gap: 4px 10px;
		font-size: 13px;
		color: $text-color-accent;

		.unmatched {
			font-style: italic;
			cursor: help;
		}
	}

	.venue {
		display: flex;
		flex-flow: row wrap;
		align-items: center;
		gap: 6px 10px;
		margin-top: 4px;

		.name {
			font-size: 13px;
			font-weight: bold;
			color: $text-color-accent;
			flex: 0 0 auto;
		}
	}

	.slots {
		display: flex;
		flex-flow: row wrap;
		gap: 6px;
	}

	.slot {
		display: flex;
		align-items: baseline;
		gap: 4px;
		padding: 5px 9px;
		border-radius: 6px;
		border: 1px solid $bg-color-accent;
		background-color: $bg-color;
		color: $text-color;
		font-size: 14px;
		font-variant-numeric: tabular-nums;
		text-decoration: none;

		em {
			font-size: 11px;
			font-style: normal;
			color: $text-color-accent;
		}
	}

	a.slot:hover,
	a.slot:focus-visible {
		background-color: $accent-color-hover;
		color: $bg-color;

		em {
			color: $bg-color;
		}
	}

	@media screen and (max-width: 500px) {
		.showtimes li {
			gap: 10px;
			padding: 10px;
		}

		.poster img,
		.poster .noposter {
			width: 58px;
			height: 87px;
		}
	}
</style>
