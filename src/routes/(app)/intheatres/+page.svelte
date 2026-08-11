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
	import PageTitle from "@/lib/generic/PageTitle.svelte";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import Error from "@/lib/Error.svelte";
	import tooltip from "@/lib/actions/tooltip";
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

	// Which set of theatres we're showing. `nearby` still needs geolocation plus
	// venues we don't have showtimes for, so it stays out.
	type Scope = "all" | "nearby" | "mine";

	const scopes: { id: Scope; label: string; ready: boolean }[] = [
		{ id: "all", label: "All", ready: true },
		{ id: "nearby", label: "Near me", ready: false },
		{ id: "mine", label: "My theatres", ready: true },
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
	const week = Array.from({ length: 7 }, (_, i) => {
		const d = new Date();
		d.setDate(d.getDate() + i);
		return {
			iso: isoDay(d),
			weekday: d.toLocaleDateString(undefined, { weekday: "short" }),
			dayNum: d.getDate(),
			isToday: i === 0,
		};
	});

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
	let mineLoading = $derived(loadingDay === selectedDay);
	let mineError = $derived(
		failedDay?.day === selectedDay ? failedDay.error : undefined,
	);

	function scopeFromUrl(): Scope {
		const s = page.url.searchParams.get("scope");
		return scopes.some((x) => x.id === s && x.ready) ? (s as Scope) : "all";
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
		const p = new URLSearchParams();
		if (activeScope !== "all") p.set("scope", activeScope);
		if (selectedDay !== week[0].iso) p.set("day", selectedDay);
		const qs = p.toString();
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

	$effect(() => {
		if (
			activeScope === "mine" &&
			!days[selectedDay] &&
			loadingDay !== selectedDay &&
			failedDay?.day !== selectedDay
		) {
			loadDay(selectedDay);
		}
	});

	let nextLoadParams: DiscoverRequest = $derived({
		page: dataLoader.state.page + 1,
		type: SearchType.movie,
		filter: DiscoverFilter.inTheatres,
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

	onMount(() => {
		dataLoader.runFn(PaginatedLoaderRunFnAction.Reset);
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
		<div class="back">
			<BackButton />
		</div>
		<PageTitle title="In Theatres">
			<div class="scopes">
				{#each scopes as s (s.id)}
					<button
						class="plain"
						data-active={activeScope === s.id}
						disabled={!s.ready}
						onclick={() => {
							activeScope = s.id;
							syncUrl();
						}}
						use:tooltip={{
							text: "Needs a showtimes source — not wired up yet.",
							pos: "bot",
							condition: !s.ready,
						}}
					>
						{s.label}
					</button>
				{/each}
			</div>
		</PageTitle>

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
					<h2 class="norm">Nothing showing!</h2>
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
	.back {
		margin: 0 15px;
	}

	.week {
		display: flex;
		flex-flow: row;
		gap: 6px;
		margin: 0 15px 16px 15px;
		overflow-x: auto;
		scrollbar-width: thin;

		button {
			display: flex;
			flex-flow: column;
			align-items: center;
			gap: 1px;
			flex: 1 1 0;
			min-width: 58px;
			padding: 7px 6px;
			border-radius: 8px;
			border: 1px solid $bg-color-accent;
			color: $text-color;

			.wd {
				font-size: 11px;
				text-transform: uppercase;
				color: $text-color-accent;
			}

			.dn {
				font-size: 17px;
				font-weight: bold;
				font-variant-numeric: tabular-nums;
			}

			&:hover {
				border-color: $text-color;
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

	.scopes {
		display: flex;
		flex-flow: row;
		flex-wrap: wrap;
		gap: 10px;
		margin-left: auto;

		button {
			padding: 8px 14px;
			border-radius: 8px;
			font-size: 14px;
			color: $text-color;
			fill: $text-color;
			transition:
				background-color 150ms ease,
				color 150ms ease,
				outline 150ms ease;

			&:disabled {
				opacity: 0.45;
				cursor: not-allowed;
			}

			&:not(:disabled):hover,
			&[data-active="true"] {
				color: $bg-color;
				fill: $bg-color;
				background-color: $accent-color-hover;
			}

			&[data-active="true"] {
				outline: 3px solid $accent-color;
			}
		}
	}

	.content {
		display: flex;
		width: 100%;
		justify-content: center;

		.inner {
			width: 100%;
			max-width: 1200px;
		}
	}
</style>
