<script module lang="ts">
	// Cached watched-list state, kept across navigations so returning to the
	// list (browser back after opening a title) restores the loaded items and
	// scroll position instead of reloading from the top.
	let listCache: {
		data: unknown[];
		page: number;
		pageMax: number;
		scrollY: number;
		key: string;
	} | null = null;
</script>

<script lang="ts">
	import { beforeNavigate, goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { applyContentUpdates, setListOrder } from "@/lib/util/listNav.svelte";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Poster from "@/lib/poster/Poster.svelte";
	import PosterList from "@/lib/poster/PosterList.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import UpNext from "@/lib/UpNext.svelte";
	import { req } from "@/lib/util/api";
	import infScroll from "@/lib/util/infScroll";
	import paginatedLoader from "@/lib/util/paginatedLoader.svelte";
	import { clearActiveFilters, setWatchedListMode, store } from "@/store.svelte";
	import { type Media, type PaginationResponse } from "@/types";
	import { onDestroy, untrack } from "svelte";

	const scroll = infScroll({ callback: onScrollToBottom });
	const dataLoader = paginatedLoader<Media, undefined>(load);

	// Cache the list (items + scroll) whenever we navigate away, so returning
	// restores the view instead of reloading from the top. Captured here while
	// still mounted, so it's ready by the time the page remounts on the way back.
	let initialLoad = true;
	beforeNavigate(() => {
		if (dataLoader.state.data.length > 0) {
			listCache = {
				data: dataLoader.state.data,
				page: dataLoader.state.page,
				pageMax: dataLoader.state.pageMax,
				scrollY: window.scrollY,
				key: JSON.stringify(store.sortAndFiltersForQueryParams),
			};
		}
	});

	let nextLoadParams: {
		page: number;
		[x: string]: unknown;
	} = $derived({
		page: dataLoader.state.page + 1,
		...store.sortAndFiltersForQueryParams,
	});

	async function load(signal: AbortSignal) {
		console.debug("load: loadParams:", nextLoadParams);
		if (nextLoadParams.page === dataLoader.state.page) {
			console.warn("load: Already on this page, not loading it again!");
			return;
		}
		const r = await req.get<PaginationResponse<Media, undefined>>(`/watched`, {
			params: nextLoadParams,
			signal,
		});
		scroll.dataLoaded();
		return r;
	}

	async function onScrollToBottom() {
		// If an error is being shown, no more infinite scroll.
		if (dataLoader.state.reqLoadError) {
			return;
		}
		dataLoader.runFn();
	}

	// Scroll back to a saved position once the (async-rendered) list is tall
	// enough to reach it — otherwise scrolling happens before the posters lay
	// out and gets clamped near the top. Re-applies once more to win over
	// SvelteKit's own early scroll restoration.
	function restoreScrollTo(y: number) {
		let tries = 0;
		const step = () => {
			const maxScroll =
				document.documentElement.scrollHeight - window.innerHeight;
			if (maxScroll >= y || tries >= 60) {
				window.scrollTo(0, y);
				requestAnimationFrame(() => window.scrollTo(0, y));
				return;
			}
			tries++;
			requestAnimationFrame(step);
		};
		requestAnimationFrame(step);
	}

	// True when the type filter is set to exactly this single media type.
	function isTypeOnly(t: string): boolean {
		return (
			store.activeFilters?.type?.length === 1 &&
			store.activeFilters.type[0] === t
		);
	}

	// NOTE: This effect also handles initial load of data.
	$effect(() => {
		// Track sort/filter changes (also performs the initial load).
		const qp = store.sortAndFiltersForQueryParams;
		untrack(() => {
			// On the first mount, if we have a cached list for this exact
			// sort/filter (i.e. we're returning to the page), restore it and its
			// scroll position instead of reloading from the top. Later runs
			// (sort/filter changes) always reload.
			if (
				initialLoad &&
				listCache &&
				listCache.key === JSON.stringify(qp) &&
				listCache.data.length > 0
			) {
				initialLoad = false;
				// Patch in any items refreshed on their detail page (e.g. newly
				// translated), leaving the rest of the cached list untouched.
				dataLoader.state.data = applyContentUpdates(
					listCache.data,
				) as Media[];
				dataLoader.state.page = listCache.page;
				dataLoader.state.pageMax = listCache.pageMax;
				restoreScrollTo(listCache.scrollY);
				return;
			}
			initialLoad = false;
			// We don't want to trigger another re-run of this
			// effect when state inside these funcs changes.
			dataLoader.reset();
			dataLoader.runFn();
		});
	});

	// Publish the current list order so detail pages can offer prev/next
	// navigation (arrows + swipe) matching what's shown here.
	$effect(() => {
		setListOrder(dataLoader.state.data);
	});

	onDestroy(() => {
		console.log("MAIN PAGE DESTROYED");
		scroll.destroy();
		dataLoader.abortReq("page destroyed");
	});
</script>

<svelte:head>
	<title>Watched List</title>
</svelte:head>

<!-- <span
	style="position: fixed; top: 80px; background-color: white; color: black; z-index: 60;"
>
	<b>listPage</b>: {dataLoader.state.page}
	listPageMax: {dataLoader.state.pageMax}
	listLoading: {dataLoader.state.reqLoading}
	<b>sort:</b>
	{JSON.stringify(store.activeSort)}
	<b>filter:</b>
	{JSON.stringify(store.activeFilters)}
	<b>queryp:</b>
	{JSON.stringify(store.sortAndFiltersForQueryParams)}
	paginatedLoader.state.meta: {JSON.stringify(dataLoader.state.meta)}
</span> -->

<div class="type-toggle">
	<div class="segments">
		<button
			class="plain"
			data-active={!store.activeFilters?.type?.length}
			onclick={() => setWatchedListMode("all")}
		>
			All
		</button>
		<button
			class="plain"
			data-active={isTypeOnly("tv")}
			onclick={() => setWatchedListMode("tv")}
		>
			<Icon i="tv" wh={18} /> TV Shows
		</button>
		<button
			class="plain"
			data-active={isTypeOnly("movie")}
			onclick={() => setWatchedListMode("movie")}
		>
			<Icon i="film" wh={18} /> Movies
		</button>
	</div>
	<!-- Not a fourth segment: the ones above filter your list, this one leaves
	     the page. Kept detached (and stateless) so the shape says so. -->
	<a class="intheatres" href={resolve("/discover?type=movie&filter=intheatres")}>
		<Icon i="ticket" wh={18} /> In Theatres
		<Icon i="chevron" facing="right" wh={13} />
	</a>
</div>

{#if !isTypeOnly("movie")}
	<UpNext />
{/if}

<PosterList>
	{#if dataLoader.state.data?.length > 0}
		{#each dataLoader.state.data as w, i (`${i}-${w.type}`)}
			{#if w}
				<Poster
					bind:watched={dataLoader.state.data[i].watched}
					media={w}
					fluidSize={true}
				/>
			{/if}
		{/each}
	{:else if !dataLoader.state.reqLoading && !dataLoader.state.reqLoadError}
		<div class="empty-list">
			<Icon i={store.hasActiveFilters ? "filter-circle" : "reel"} wh={80} />
			<h2 class="norm">Your list looks empty!</h2>
			<h4 class="norm">
				Try {`${store.hasActiveFilters ? "removing your active filters or" : ""}`}
				searching for something you would like to add.
			</h4>
			{#if !store.hasActiveFilters}
				<button onclick={() => goto(resolve("/import"))}>Import</button>
			{/if}
			{#if store.hasActiveFilters}
				<button onclick={() => clearActiveFilters()}>Clear Filters</button>
			{/if}
		</div>
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
			pretty="Failed to load results!"
			error={dataLoader.state.reqLoadError}
			onRetry={() => {
				dataLoader.state.reqLoadError = undefined;
				dataLoader.runFn();
			}}
		/>
	</div>
{/if}

<!-- TODO: A 'That's it' message when you reach bottom of your list? -->
<!-- {#if !dataLoader.state.reqLoadError && dataLoader.state.page === dataLoader.state.pageMax}
	<b>That's it!</b>
{/if} -->

<style lang="scss">
	.type-toggle {
		display: flex;
		flex-flow: row;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px 26px;
		justify-content: center;
		margin: 0 auto 15px auto;

		.segments {
			display: flex;
			flex-flow: row;
			flex-wrap: wrap;
			gap: 10px;
			justify-content: center;
		}

		.intheatres {
			display: flex;
			flex-flow: row;
			align-items: center;
			gap: 6px;
			padding: 8px 12px;
			border-radius: 8px;
			font-size: 14px;
			font-weight: bold;
			text-decoration: none;
			opacity: 0.75;
			color: $text-color;
			fill: $text-color;
			transition:
				opacity 150ms ease,
				background-color 150ms ease;

			&:hover,
			&:focus-visible {
				opacity: 1;
				background-color: $accent-color;
			}
		}

		button {
			display: flex;
			flex-flow: row;
			align-items: center;
			gap: 8px;
			padding: 8px 14px;
			border-radius: 8px;
			font-size: 14px;
			color: $text-color;
			fill: $text-color;
			transition:
				background-color 150ms ease,
				color 150ms ease,
				outline 150ms ease;

			&:hover,
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

	.empty-list {
		display: flex;
		flex-flow: column;
		gap: 5px;
		align-items: center;
		max-width: 400px;

		h2 {
			margin-top: 10px;
		}

		h4 {
			font-weight: normal;
			text-align: center;
		}

		button {
			width: max-content;
			padding-left: 20px;
			padding-right: 20px;
			margin-top: 15px;
		}
	}
</style>
