<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import LiveShowCard from "@/lib/live/LiveShowCard.svelte";
	import { listPeople, listShows } from "@/lib/live/api";
	import type { LivePersonSummary, LiveShow } from "@/types";

	type Tab = "upcoming" | "seen" | "artists";
	const tabs: { id: Tab; label: string }[] = [
		{ id: "upcoming", label: "Upcoming" },
		{ id: "seen", label: "Seen" },
		{ id: "artists", label: "Artists" },
	];
	const defaultTab: Tab = "upcoming";

	// The tab lives in the URL (replaced, not pushed), so Back from a show or
	// an artist returns to the tab it was opened from.
	let tab: Tab = $derived.by(() => {
		const t = page.url.searchParams.get("tab");
		return tabs.some((x) => x.id === t) ? (t as Tab) : defaultTab;
	});

	function selectTab(t: Tab) {
		// Two literal branches: `resolve` is typed against the route table.
		goto(t === defaultTab ? resolve("/live") : resolve(`/live?tab=${t}`), {
			replaceState: true,
			keepFocus: true,
			noScroll: true,
		});
	}

	let people: LivePersonSummary[] | undefined = $state();
	let peopleLoading = $state(false);
	let peopleError: unknown = $state();

	async function loadPeople() {
		peopleLoading = true;
		peopleError = undefined;
		try {
			people = await listPeople();
		} catch (err) {
			console.error("live: load artists failed", err);
			peopleError = err;
		} finally {
			peopleLoading = false;
		}
	}

	// Only fetched once the Artists tab is opened.
	$effect(() => {
		if (tab === "artists" && !people && !peopleLoading && !peopleError)
			loadPeople();
	});

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

		<div class="tabs" role="tablist" aria-label="Live shows">
			{#each tabs as t (t.id)}
				<button
					class="plain"
					role="tab"
					aria-selected={tab === t.id}
					data-active={tab === t.id}
					onclick={() => selectTab(t.id)}
				>
					{t.label}
				</button>
			{/each}
		</div>

		{#if tab === "artists"}
			{#if peopleError}
				<Error
					pretty="Failed to load your artists!"
					error={peopleError}
					onRetry={() => {
						loadPeople();
					}}
				/>
			{:else if !people}
				<Spinner />
			{:else if people.length}
				<ul class="people">
					{#each people as p (p.id)}
						<li>
							<a href={resolve(`/live/people/${p.id}`)}>
								<span class="name">{p.name}</span>
								<span class="count">
									{p.showCount} show{p.showCount === 1 ? "" : "s"}
								</span>
							</a>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="empty">
					No artists yet. Add who was in a show from its Edit form.
				</p>
			{/if}
		{:else if loading && !shows}
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
			{#if tab === "upcoming"}
				{#if upcoming.length}
					<div class="list">
						{#each upcoming as s (s.id)}
							<LiveShowCard show={s} />
						{/each}
					</div>
				{:else}
					<p class="empty">Nothing booked yet.</p>
				{/if}
			{:else if seen.length}
				<div class="list">
					{#each seen as s (s.id)}
						<LiveShowCard show={s} />
					{/each}
				</div>
			{:else}
				<p class="empty">No shows seen yet.</p>
			{/if}
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

	// Same look as In Theatres' scope buttons.
	.tabs {
		display: flex;
		flex-flow: row nowrap;
		gap: 8px;
		margin: 8px 0 16px 0;
		overflow-x: auto;
		scrollbar-width: none;

		button {
			flex: 0 0 auto;
			padding: 8px 14px;
			border-radius: 8px;
			border: 1px solid $bg-color-accent;
			font-size: 14px;
			color: $text-color;
			transition:
				background-color 150ms ease,
				color 150ms ease;

			// Hover only where hovering exists: on touch it sticks to the last
			// tapped button, which would then look selected.
			@media (hover: hover) {
				&:hover {
					background-color: $bg-color-accent;
				}
			}

			&[data-active="true"] {
				color: $bg-color;
				background-color: $accent-color-hover;
				border-color: $accent-color-hover;
			}
		}
	}

	.people {
		display: flex;
		flex-flow: column;
		margin: 0;
		padding: 0;
		list-style: none;

		li + li {
			border-top: 1px solid $bg-color-accent;
		}

		a {
			display: flex;
			align-items: center;
			gap: 10px;
			padding: 12px 6px;
			color: $text-color;

			.name {
				flex: 1 1 auto;
				min-width: 0;
				font-weight: bold;
				overflow-wrap: anywhere;
			}

			.count {
				flex: 0 0 auto;
				font-size: 14px;
				color: $text-color-accent;
			}

			@media (hover: hover) {
				&:hover .name {
					text-decoration: underline;
				}
			}

			&:focus-visible .name {
				text-decoration: underline;
			}
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
