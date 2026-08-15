<script module lang="ts">
	import type { TheatresResponse as CachedResponse } from "@/types";

	// Kept across navigations so returning to this listing (Back from a film)
	// restores the days already fetched and the scroll position, instead of
	// refetching and dropping you at the top.
	let cache: {
		days: Record<string, CachedResponse>;
		scrollY: number;
		// Which view the scroll belongs to; restoring it onto a different day or
		// scope would land somewhere arbitrary.
		key: string;
	} | null = null;
</script>

<script lang="ts">
	import Spinner from "@/lib/Spinner.svelte";
	import { req } from "@/lib/util/api";
	import Poster from "@/lib/poster/Poster.svelte";
	import PosterList from "@/lib/poster/PosterList.svelte";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import Error from "@/lib/Error.svelte";
	import infScroll from "@/lib/util/infScroll";
	import paginatedLoader, {
		PaginatedLoaderRunFnAction,
	} from "@/lib/util/paginatedLoader.svelte";
	import ShowtimesList from "@/lib/theatres/ShowtimesList.svelte";
	import {
		DiscoverFilter,
		SearchType,
		type DiscoverRequest,
		type Media,
		type PaginationResponse,
		type TheatresResponse,
	} from "@/types";
	import { onDestroy, onMount } from "svelte";
	import { page } from "$app/state";
	import { beforeNavigate, goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { restoreScrollTo } from "@/lib/util/restoreScroll";

	// Which set of theatres we're showing.
	//
	// There was a "Near me" scope here. It's gone for now: the showtimes source
	// only covers cinemas running the Erakys platform, and the other Marseille
	// venues (Le Prado, Le Chambord) don't — so "near me" would have listed
	// nothing the "My theatres" scope doesn't already show. See DEV-WATCHARR.md.
	type Scope = "mine" | "week" | "next";

	// Own cinemas first: it's the default, and putting the default anywhere but
	// first makes the row read as if something else were selected.
	const scopes: { id: Scope; label: string }[] = [
		{ id: "mine", label: "My theatres" },
		{ id: "week", label: "This week" },
		{ id: "next", label: "Next week" },
	];

	const scroll = infScroll({ callback: onScrollToBottom });
	const dataLoader = paginatedLoader<Media, undefined>(load);

	/** Local YYYY-MM-DD. Not toISOString(), which shifts to UTC and can land on
	 *  the wrong day for anyone east of Greenwich. */
	function isoDay(d: Date) {
		return [
			d.getFullYear(),
			String(d.getMonth() + 1).padStart(2, "0"),
			String(d.getDate()).padStart(2, "0"),
		].join("-");
	}

	// Today plus six: cinema weeks run Wednesday to Tuesday, so a fixed
	// Monday-to-Sunday strip would cut the current programme in half.
	// Declared before the state below, which reads it to validate the URL.
	const today = new Date();
	const week = Array.from({ length: 7 }, (_, i) => {
		// Day arithmetic via the constructor's overflow, which rolls months and
		// years over and stays on local calendar days across a DST change —
		// unlike adding 86400000ms.
		const d = new Date(
			today.getFullYear(),
			today.getMonth(),
			today.getDate() + i,
		);
		return {
			iso: isoDay(d),
			weekday: d.toLocaleDateString(undefined, { weekday: "short" }),
			dayNum: d.getDate(),
			isToday: i === 0,
		};
	});

	// "My theatres" is the default: it's the reason to open this page. "All" is
	// the wider browse you fall back to, so it's the one that names itself in
	// the URL.
	// Declared before the state below, which reads it through scopeFromUrl().
	const defaultScope: Scope = "mine";

	// Scope and day live in the URL so coming back to this page — browser back,
	// the in-app Back button, a reload or a shared link — restores what you were
	// actually looking at instead of resetting to "All, today".
	let activeScope: Scope = $state(scopeFromUrl());

	// `mine` comes from a different endpoint with no pagination, so it gets its
	// own state rather than being forced through the paginated loader.
	//
	// Days are fetched one at a time, on demand, and kept. Pulling the whole week
	// up front would mean 14 scrapes of two small cinemas' sites per refresh, to
	// show six days you probably won't look at.
	let days: Record<string, TheatresResponse> = $state(cache?.days ?? {});
	let selectedDay = $state(dayFromUrl());
	let loadingDay: string | undefined = $state();
	let failedDay: { day: string; error: unknown } | undefined = $state();

	let mine = $derived(days[selectedDay]);
	// Only a spinner when there's nothing to show. A refresh over data we
	// already have happens quietly underneath it.
	let mineLoading = $derived(loadingDay === selectedDay && !days[selectedDay]);
	// Likewise, a failed refresh must not replace a listing that's already on
	// screen with an error box. Only report the failure when we have nothing.
	let mineError = $derived(
		failedDay?.day === selectedDay && !days[selectedDay]
			? failedDay.error
			: undefined,
	);

	function scopeFromUrl(): Scope {
		const s = page.url.searchParams.get("scope");
		// `all` was this page's earlier name for the current week's releases;
		// keep older links working.
		if (s === "all") return "week";
		return scopes.some((x) => x.id === s) ? (s as Scope) : defaultScope;
	}

	function dayFromUrl(): string {
		const d = page.url.searchParams.get("day");
		// Only accept a day we actually offer, so a stale or hand-edited link
		// can't ask the cinemas for 1998.
		return d && week.some((w) => w.iso === d) ? d : week[0].iso;
	}

	/**
	 * Mirror scope and day into the URL, replacing the entry rather than pushing:
	 * flipping through seven days shouldn't leave seven steps for Back to unwind.
	 */
	function syncUrl() {
		// Built by hand rather than with URLSearchParams: two params whose values
		// are a fixed scope id and an ISO date, so there is nothing to escape.
		const parts: string[] = [];
		if (activeScope !== defaultScope) parts.push(`scope=${activeScope}`);
		if (selectedDay !== week[0].iso) parts.push(`day=${selectedDay}`);
		const qs = parts.join("&");
		// Two literal branches: `resolve` is typed against the route table and
		// rejects a template whose prefix isn't a literal path.
		goto(qs ? resolve(`/intheatres?${qs}`) : resolve("/intheatres"), {
			replaceState: true,
			keepFocus: true,
			noScroll: true,
		});
	}

	async function loadDay(day: string) {
		loadingDay = day;
		if (failedDay?.day === day) failedDay = undefined;
		try {
			days[day] = await req.get<TheatresResponse>("/theatres/showtimes", {
				params: { day },
			});
		} catch (err) {
			console.error("loadDay failed", day, err);
			failedDay = { day, error: err };
		} finally {
			if (loadingDay === day) loadingDay = undefined;
		}
	}

	// Days already fetched are shown straight away, then refreshed underneath —
	// otherwise the cache that preserves your scroll position would also freeze
	// the listing for the whole session, so marking a film watched elsewhere
	// would never show up here. Once per mount per day is enough: the server
	// caches the programme for hours anyway.
	// A plain object, not $state (nor a Set, which the reactivity lint rule
	// would push towards SvelteSet): writing to it must not re-run this effect.
	const revalidated: Record<string, true> = {};
	$effect(() => {
		if (
			activeScope === "mine" &&
			!revalidated[selectedDay] &&
			loadingDay !== selectedDay &&
			failedDay?.day !== selectedDay
		) {
			revalidated[selectedDay] = true;
			loadDay(selectedDay);
		}
	});

	let discoverFilter = $derived(
		activeScope === "next"
			? DiscoverFilter.inTheatresNext
			: DiscoverFilter.inTheatres,
	);

	let nextLoadParams: DiscoverRequest = $derived({
		page: dataLoader.state.page + 1,
		type: SearchType.movie,
		filter: discoverFilter,
	});

	async function load(signal: AbortSignal) {
		if (nextLoadParams.page === dataLoader.state.page) {
			console.warn("load: Already on this page, not loading it again!");
			return;
		}
		const r = await req.get<PaginationResponse<Media, undefined>>(`/discover`, {
			params: nextLoadParams,
			signal,
		});
		scroll.dataLoaded();
		return r;
	}

	async function onScrollToBottom() {
		if (dataLoader.state.reqLoadError) {
			return;
		}
		dataLoader.runFn();
	}

	let viewKey = $derived(`${activeScope}|${selectedDay}`);

	// Capture while still mounted, so it's ready by the time we come back.
	beforeNavigate(() => {
		cache = { days, scrollY: window.scrollY, key: viewKey };
	});

	// The two release listings share one paginated loader, so switching between
	// them has to reset it — otherwise page 2 of "next week" would be appended
	// to page 1 of "this week". Loaded lazily too: neither is the default scope,
	// so fetching on mount would cost a TMDB page per visit for nothing.
	// A plain variable, not $state: it must not re-trigger this effect.
	let loadedScope: Scope | undefined;
	$effect(() => {
		if (activeScope !== "mine" && loadedScope !== activeScope) {
			loadedScope = activeScope;
			dataLoader.abortReq("scope changed");
			dataLoader.runFn(PaginatedLoaderRunFnAction.Reset);
		}
	});

	onMount(() => {
		// Only restore onto the same view the scroll was taken from. Rows have
		// fixed-height posters, so the list's height doesn't wait on images.
		if (cache && cache.key === viewKey) {
			restoreScrollTo(cache.scrollY);
		}
	});

	onDestroy(() => {
		scroll.destroy();
		dataLoader.abortReq("page destroyed");
	});
</script>

<svelte:head>
	<title>In Theatres</title>
</svelte:head>

<div class="content">
	<div class="inner">
		<!-- One row: Back left, scopes right. The page title was dropped — the
		     scopes already say where you are, and on a phone that row was 4% of the
		     screen before a single film appeared. -->
		<div class="head">
			<BackButton />
			<!-- Scrolls sideways rather than wrapping, so a narrow screen keeps the
			     row intact instead of spending a second line on it. -->
			<div class="scopes">
				{#each scopes as s (s.id)}
					<button
						class="plain"
						data-active={activeScope === s.id}
						onclick={() => {
							activeScope = s.id;
							syncUrl();
						}}
					>
						{s.label}
					</button>
				{/each}
			</div>
		</div>

		{#if activeScope === "mine"}
			<div class="week">
				{#each week as d (d.iso)}
					<button
						class="plain"
						data-active={selectedDay === d.iso}
						onclick={() => {
							selectedDay = d.iso;
							syncUrl();
						}}
					>
						<span class="wd">{d.isToday ? "Today" : d.weekday}</span>
						<span class="dn">{d.dayNum}</span>
					</button>
				{/each}
			</div>

			{#if mine?.films?.length}
				<ShowtimesList
					films={mine.films}
					theatres={mine.theatres}
					showSummary
				/>
			{:else if !mineLoading && !mineError}
				<h2 class="norm">
					{selectedDay === week[0].iso
						? "Nothing left today at your theatres!"
						: "Nothing programmed yet for that day."}
				</h2>
			{/if}

			{#if mineLoading}
				<div style="margin-bottom: 60px;"><Spinner /></div>
			{/if}

			{#if mineError}
				<div style="margin-bottom: 60px;">
					<Error
						pretty="Failed to load your theatres' showtimes!"
						error={mineError}
						onRetry={() => {
							loadDay(selectedDay);
						}}
					/>
				</div>
			{/if}
		{:else}
			<PosterList>
				{#if dataLoader.state.data?.length > 0}
					{#each dataLoader.state.data as w, i (`${i}-${w.type}`)}
						<Poster
							media={w}
							bind:watched={dataLoader.state.data[i].watched}
							fluidSize
							showExternalRating
						/>
					{/each}
				{:else if !dataLoader.state.reqLoading && !dataLoader.state.reqLoadError}
					<h2 class="norm">
						{activeScope === "next"
							? "Nothing announced for next week yet!"
							: "Nothing showing!"}
					</h2>
				{/if}
			</PosterList>

			{#if dataLoader.state.reqLoading}
				<div style="margin-bottom: 60px;">
					<Spinner />
				</div>
			{/if}

			{#if dataLoader.state.reqLoadError}
				<div style="margin-bottom: 60px;">
					<Error
						pretty="Failed to load what's showing!"
						error={dataLoader.state.reqLoadError}
						onRetry={() => {
							dataLoader.state.reqLoadError = undefined;
							dataLoader.runFn(
								PaginatedLoaderRunFnAction.ResetIfOnFirstOrNoPage,
							);
						}}
					/>
				</div>
			{/if}
		{/if}
	</div>
</div>

<style lang="scss">
	/* Align with PageTitle, which carries its own 15px side margin. */
	.head {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0 15px 10px 15px;

		// BackButton carries its own bottom margin for the detail pages; here it
		// shares a row, so it must not push the row apart. The selector includes
		// the tag to outrank the component's own rule — at equal specificity the
		// winner depends on style order.
		:global(button.back-button) {
			margin-bottom: 0;
			flex: 0 0 auto;
		}

		// Back carries a 2px border the scope buttons don't, so matching their
		// paddings still left it taller. Pin both to one height instead and
		// centre the contents — that survives any later padding tweak.
		:global(button.back-button),
		.scopes button {
			display: flex;
			align-items: center;
			height: 38px;
			padding-top: 0;
			padding-bottom: 0;
		}
	}

	.scopes {
		display: flex;
		flex-flow: row nowrap;
		gap: 8px;
		min-width: 0;
		margin-left: auto;
		overflow-x: auto;
		scrollbar-width: none;
		// `safe` so that when the row does overflow, right-alignment can't push
		// the first button past the scroll origin, out of reach.
		justify-content: flex-end;
		justify-content: safe flex-end;

		button {
			flex: 0 0 auto;
			padding: 8px 14px;
			border-radius: 8px;
			// Outlined like the day chips below. Without it only the selected
			// button had an edge, so the others' padding was invisible and the
			// spacing read as uneven — and the two rows looked unrelated.
			border: 1px solid $bg-color-accent;
			font-size: 14px;
			color: $text-color;
			fill: $text-color;
			transition:
				background-color 150ms ease,
				color 150ms ease,
				outline 150ms ease;

			// Hover only where hovering exists. On touch, :hover sticks to
			// whatever was last tapped — including the spot the previous page's
			// link happened to occupy — so a second button would look selected.
			@media (hover: hover) {
				&:hover {
					color: $bg-color;
					fill: $bg-color;
					background-color: $accent-color-hover;
				}
			}

			// Same selected treatment as the day chips: fill the chip rather
			// than ring it. The outline sat outside the box and made the
			// selected button stand taller than its neighbours.
			&[data-active="true"] {
				color: $bg-color;
				fill: $bg-color;
				background-color: $accent-color-hover;
				border-color: $accent-color-hover;
			}
		}
	}

	.week {
		display: flex;
		flex-flow: row;
		gap: 6px;
		margin: 0 15px 12px 15px;
		overflow-x: auto;
		scrollbar-width: thin;

		button {
			// One line per day rather than two: the strip already scrolls, so
			// spending a second line to stack the weekday above the number was
			// pure height. Chips size to their content instead of sharing the
			// width equally, which keeps them compact.
			display: flex;
			flex-flow: row;
			align-items: baseline;
			gap: 5px;
			flex: 0 0 auto;
			padding: 6px 11px;
			border-radius: 8px;
			border: 1px solid $bg-color-accent;
			color: $text-color;

			.wd {
				font-size: 12px;
				color: $text-color-accent;
			}

			.dn {
				font-size: 15px;
				font-weight: bold;
				font-variant-numeric: tabular-nums;
			}

			@media (hover: hover) {
				&:hover {
					border-color: $text-color;
				}
			}

			&[data-active="true"] {
				background-color: $accent-color-hover;
				color: $bg-color;
				border-color: $accent-color-hover;

				.wd {
					color: $bg-color;
				}
			}
		}
	}

	.content {
		display: flex;
		width: 100%;
		justify-content: center;
		// The nav leaves 20px below itself for every page. This one is a dense
		// header, so it claws some of it back rather than starting a third of
		// the way down a phone screen.
		margin-top: -10px;

		.inner {
			width: 100%;
			max-width: 1200px;
		}
	}
</style>
