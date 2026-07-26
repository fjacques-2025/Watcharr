<script module lang="ts">
	// Cached watched-list state, kept across navigations so returning to the
	// list (e.g. via the browser back button after opening a title) restores
	// the loaded items and scroll position instead of reloading from the top.
	let listCache: {
		data: any[];
		page: number;
		pageMax: number;
		scrollY: number;
		key: string;
	} | null = null;
</script>

<script lang="ts">
	import { beforeNavigate, goto } from "$app/navigation";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Poster from "@/lib/poster/Poster.svelte";
	import PosterList from "@/lib/poster/PosterList.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import UpNext from "@/lib/UpNext.svelte";
	import infScroll from "@/lib/util/infScroll";
	import paginatedLoader from "@/lib/util/paginatedLoader.svelte";
	import { clearActiveFilters, setWatchedListMode, store } from "@/store.svelte";
	import { setListOrder } from "@/lib/util/listNav.svelte";
	import type { Media } from "@/types";
	import axios, { type GenericAbortSignal } from "axios";
	import { onDestroy, untrack } from "svelte";

	const scroll = infScroll({ callback: onScrollToBottom });
	const dataLoader = paginatedLoader<Media, undefined>(load);

	// Cache the list (items + scroll) whenever we navigate away, so returning
	// restores the view instead of reloading from the top. Captured here (while
	// still mounted) rather than via a snapshot export, so it's ready and
	// deterministic by the time the page remounts on the way back.
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
		[x: string]: any;
	} = $derived({
		page: dataLoader.state.page + 1,
		...store.sortAndFiltersForQueryParams,
	});

	async function load(signal: GenericAbortSignal) {
		console.debug("load: loadParams:", nextLoadParams);
		if (nextLoadParams.page === dataLoader.state.page) {
			console.warn("load: Already on this page, not loading it again!");
			return;
		}
		const r = await axios.get(`/watched`, {
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

	// After restoring the cached list, silently re-fetch the same range (one
	// request, limit = items shown) and swap in fresh content — so updates made
	// while away (e.g. a title translated on its detail page) show up without
	// changing the item count, keeping the scroll position stable.
	async function refreshLoadedInPlace(key: string) {
		try {
			const count = dataLoader.state.data.length;
			if (count <= 0) return;
			const r = await axios.get(`/watched`, {
				params: { page: 1, limit: count, ...store.sortAndFiltersForQueryParams },
			});
			// Bail if the sort/filter changed while we were fetching.
			if (JSON.stringify(store.sortAndFiltersForQueryParams) !== key) return;
			const fresh = r?.data?.results;
			if (fresh && fresh.length > 0) {
				dataLoader.state.data = fresh;
			}
		} catch (err) {
			console.warn("refreshLoadedInPlace: failed", err);
		}
	}

	// Scroll back to a saved position once the (async-rendered) list is tall
	// enough to reach it — otherwise scrolling happens before the posters lay
	// out and gets clamped near the top. Also wins over SvelteKit's own early
	// scroll restoration by re-applying on the next frame.
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
		return store.activeFilters?.type?.length === 1 && store.activeFilters.type[0] === t;
	}

	// NOTE: This effect also handles initial load of data.
	$effect(() => {
		// Track sort/filter changes (also performs the initial load).
		const qp = store.sortAndFiltersForQueryParams;
		untrack(() => {
			// On the first mount, if we have a cached list for this exact
			// sort/filter (i.e. we're returning to the page), restore it and its
			// scroll position instead of reloading from the top. Sort/filter
			// changes (later effect runs) always reload.
			if (
				initialLoad &&
				listCache &&
				listCache.key === JSON.stringify(qp) &&
				listCache.data.length > 0
			) {
				initialLoad = false;
				dataLoader.state.data = listCache.data;
				dataLoader.state.page = listCache.page;
				dataLoader.state.pageMax = listCache.pageMax;
				restoreScrollTo(listCache.scrollY);
				refreshLoadedInPlace(listCache.key);
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

<!-- <span style="position: fixed; top: 80px; background-color: white; z-index: 60;"
	><b>listPage</b>: {dataLoader.state.page} listPageMax: {dataLoader.state
		.pageMax} listLoading:
	{dataLoader.state.reqLoading}
	<b>sort:</b>
	{JSON.stringify(store.activeSort)} <b>filter:</b>
	{JSON.stringify(store.activeFilters)} <b>queryp:</b>
	{JSON.stringify(store.sortAndFiltersForQueryParams)}</span
> -->

<div class="type-toggle">
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
				<button onclick={() => goto("/import")}>Import</button>
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
		gap: 10px;
		justify-content: center;
		margin: 0 auto 15px auto;

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
