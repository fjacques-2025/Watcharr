<script lang="ts">
	import { resolve } from "$app/paths";
	import Icon from "@/lib/Icon.svelte";
	import type { TheatreFilm, TheatreShowtime } from "@/types";

	interface Props {
		films: TheatreFilm[];
		/** Venue names, in the order they should appear as sections. */
		theatres: string[];
		/** Show the film's synopsis in each row. */
		showSummary?: boolean;
	}

	let { films, theatres, showSummary = false }: Props = $props();

	// Pivot from film-first (how the API groups it, so a film playing at both
	// venues is matched to TMDB once) to venue-first, which is how you actually
	// decide: you pick the cinema, then look at what's on.
	//
	// A film screening at both venues therefore appears in both sections. That is
	// the point of grouping by cinema, not a duplicate.
	let sections = $derived.by(() => {
		const byVenue = theatres.map((theatre) => ({
			theatre,
			rows: [] as { film: TheatreFilm; showtimes: TheatreShowtime[] }[],
		}));
		for (const film of films) {
			for (const sc of film.screenings) {
				const section = byVenue.find((s) => s.theatre === sc.theatre);
				if (section) {
					section.rows.push({ film, showtimes: sc.showtimes });
				}
			}
		}
		// Order within a section by that venue's own programme, not the global
		// one: on a cinema's board, the film with four slots leads the day.
		for (const s of byVenue) {
			s.rows.sort(
				(a, b) =>
					b.showtimes.length - a.showtimes.length ||
					(a.showtimes[0]?.start ?? "").localeCompare(
						b.showtimes[0]?.start ?? "",
					),
			);
		}
		return byVenue.filter((s) => s.rows.length > 0);
	});

	function runtimeLabel(mins?: number) {
		if (!mins) return "";
		const h = Math.floor(mins / 60);
		return h > 0 ? `${h}h${String(mins % 60).padStart(2, "0")}` : `${mins}min`;
	}
</script>

<!-- A day's programme is a schedule, not a wall of posters: grouped by cinema,
     one row per film, showtimes as the payload. -->
{#each sections as section (section.theatre)}
	<section class="venue-section">
		<header>
			<h2>{section.theatre}</h2>
			<span class="count">
				{section.rows.length} film{section.rows.length > 1 ? "s" : ""}
			</span>
		</header>

		<ul class="showtimes">
			{#each section.rows as { film: f, showtimes } (f.title)}
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
							{#if f.telerama}
								<!-- Their scale is 0-4 "T"s. On mobile this link opens the
								     Télérama app, where a subscriber gets the full review. -->
								<a
									class="telerama"
									href={f.telerama.url}
									target="_blank"
									rel="noreferrer noopener"
									title={`Télérama: ${f.telerama.ratingLabel} (${f.telerama.rating}/4) — read the review`}
								>
									{#if f.telerama.rating > 0}
										<span class="ts" aria-hidden="true"
											>{"T".repeat(f.telerama.rating)}</span
										>
									{/if}
									<span class="label">{f.telerama.ratingLabel}</span>
								</a>
							{/if}
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

						{#if showSummary && f.media?.summary}
							<p class="summary">{f.media.summary}</p>
						{/if}

						<div class="slots">
							{#each showtimes as st (st.start + (st.bookingUrl ?? ""))}
								{@const label = st.end
									? `${st.start} – ends ${st.end}`
									: st.start}
								{#if st.bookingUrl}
									<!-- External URL: the cinema's own booking flow, so no
									     resolve() — which is why eslint flags this line. -->
									<a
										class="slot"
										href={st.bookingUrl}
										target="_blank"
										rel="noreferrer noopener"
										title={`Book ${f.title} at ${section.theatre}, ${label}`}
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
				</li>
			{/each}
		</ul>
	</section>
{/each}

<style lang="scss">
	.venue-section {
		margin: 0 0 26px 0;

		header {
			display: flex;
			flex-flow: row;
			align-items: baseline;
			gap: 10px;
			margin: 0 15px 10px 15px;
			padding-bottom: 6px;
			border-bottom: 2px solid $accent-color;

			h2 {
				font-size: 19px;
				margin: 0;
			}

			.count {
				font-size: 13px;
				color: $text-color-accent;
				margin-left: auto;
				flex: 0 0 auto;
			}
		}
	}

	.showtimes {
		display: flex;
		flex-flow: column;
		gap: 12px;
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

	.summary {
		margin: 0;
		font-size: 13px;
		line-height: 1.45;
		color: $text-color-accent;
		/* Clamp: a synopsis is context here, not the content — the showtimes are.
		   Full text is one click away on the film page. */
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.telerama {
		display: inline-flex;
		align-items: baseline;
		gap: 5px;
		text-decoration: none;
		color: $text-color;

		.ts {
			font-weight: bold;
			letter-spacing: 1px;
			/* Télérama's own mark is a red T; keep the association without
			   pretending to reproduce their logo. */
			color: #e2001a;
		}

		.label {
			color: $text-color-accent;
		}

		&:hover,
		&:focus-visible {
			.label {
				color: $text-color;
				text-decoration: underline;
			}
		}
	}

	.slots {
		display: flex;
		flex-flow: row wrap;
		gap: 6px;
		margin-top: 2px;
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

		.summary {
			-webkit-line-clamp: 2;
			line-clamp: 2;
		}
	}
</style>
