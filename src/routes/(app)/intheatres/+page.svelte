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

	let activeScope: Scope = $state("all");

	// `mine` comes from a different endpoint with no pagination, so it gets its
	// own state rather than being forced through the paginated loader.
	let mine: TheatresResponse | undefined = $state();
	let mineLoading = $state(false);
	let mineError: unknown = $state();

	async function loadMine() {
		mineLoading = true;
		mineError = undefined;
		try {
			mine = await req.get<TheatresResponse>("/theatres/showtimes");
		} catch (err) {
			console.error("loadMine failed", err);
			mineError = err;
		} finally {
			mineLoading = false;
		}
	}

	$effect(() => {
		if (activeScope === "mine" && !mine && !mineLoading && !mineError) {
			loadMine();
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

	onMount(() => {
		dataLoader.runFn(PaginatedLoaderRunFnAction.Reset);
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
						onclick={() => (activeScope = s.id)}
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
			{#if mine?.films?.length}
				<p class="day">
					{mine.theatres.join(" · ")} — today
				</p>
				<ShowtimesList films={mine.films} />
			{:else if !mineLoading && !mineError}
				<h2 class="norm">Nothing left today at your theatres!</h2>
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
							loadMine();
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

	.day {
		margin: 0 15px 10px 15px;
		font-size: 13px;
		color: $text-color-accent;
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
