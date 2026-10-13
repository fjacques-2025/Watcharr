<script lang="ts">
	import { onMount } from "svelte";
	import { resolve } from "$app/paths";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import LiveShowCard from "@/lib/live/LiveShowCard.svelte";
	import { listShows } from "@/lib/live/api";
	import type { LiveShow } from "@/types";

	let shows: LiveShow[] | undefined = $state();
	let loading = $state(false);
	let loadError: unknown = $state();

	// The server sends them date-ascending, which is already right for what's
	// coming; the history reads most recent first.
	let upcoming = $derived(shows?.filter((s) => s.state === "upcoming") ?? []);
	let seen = $derived(
		(shows?.filter((s) => s.state !== "upcoming") ?? []).sort(
			(a, b) => new Date(b.date).getTime() - new Date(a.date).getTime(),
		),
	);

	async function load() {
		loading = true;
		loadError = undefined;
		try {
			shows = await listShows();
		} catch (err) {
			console.error("live: load failed", err);
			loadError = err;
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>Live shows</title>
</svelte:head>

<div class="content">
	<div class="inner">
		<div class="head">
			<BackButton />
			<h2>Live shows</h2>
			<a class="btn add" href={resolve("/live/new")}>
				<Icon i="add" wh={16} /> Add a show
			</a>
			<a class="btn add-several" href={resolve("/live/bulk")}>Add several</a>
		</div>

		{#if loading && !shows}
			<Spinner />
		{:else if loadError}
			<Error
				pretty="Failed to load your shows!"
				error={loadError}
				onRetry={() => {
					load();
				}}
			/>
		{:else if shows}
			<section>
				<h3>Upcoming</h3>
				{#if upcoming.length}
					<div class="list">
						{#each upcoming as s (s.id)}
							<LiveShowCard show={s} />
						{/each}
					</div>
				{:else}
					<p class="empty">Nothing booked yet.</p>
				{/if}
			</section>

			<section>
				<h3>Seen</h3>
				{#if seen.length}
					<div class="list">
						{#each seen as s (s.id)}
							<LiveShowCard show={s} />
						{/each}
					</div>
				{:else}
					<p class="empty">No shows seen yet.</p>
				{/if}
			</section>
		{/if}
	</div>
</div>

<style lang="scss">
	.content {
		display: flex;
		width: 100%;
		justify-content: center;
		// Same reclaim as In Theatres: the nav's padding alone separates the two.
		margin-top: -18px;

		.inner {
			width: 100%;
			max-width: 800px;
			padding: 0 15px 60px 15px;
		}
	}

	.head {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-bottom: 10px;

		// BackButton's own bottom margin is for stacking above content; here
		// it shares a row.
		:global(button.back-button) {
			margin-bottom: 0;
			flex: 0 0 auto;
		}

		h2 {
			min-width: 0;
		}

		.add {
			gap: 6px;
			width: auto;
			margin-left: auto;
			flex: 0 0 auto;
			white-space: nowrap;
		}

		.add-several {
			width: auto;
			flex: 0 0 auto;
			white-space: nowrap;
			font-size: 14px;
		}
	}

	section {
		margin-top: 18px;

		h3 {
			margin-bottom: 8px;
		}
	}

	.list {
		display: flex;
		flex-flow: column;
		gap: 8px;
	}

	.empty {
		color: $text-color-accent;
		font-size: 14px;
	}
</style>
