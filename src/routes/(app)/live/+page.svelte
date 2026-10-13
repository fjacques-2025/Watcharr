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
	import { defaultRoles } from "@/lib/live/LiveCreditsEditor.svelte";
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

	// Usual programme order (Performer, Writer, Director…), then any other
	// role alphabetically: the same as the credits editor's chips.
	function orderRoles(roles: string[]) {
		const known = defaultRoles.filter((r) => roles.includes(r));
		const other = roles.filter((r) => !defaultRoles.includes(r)).sort();
		return [...known, ...other];
	}

	// Roles to filter on, with how many artists hold each.
	let roleCounts = $derived.by(() => {
		const counts: Record<string, number> = {};
		for (const p of people ?? [])
			for (const r of p.roles) counts[r] = (counts[r] ?? 0) + 1;
		return orderRoles(Object.keys(counts)).map((r) => ({
			role: r,
			n: counts[r],
		}));
	});

	// Like the tab, the filter lives in the URL so Back from an artist keeps it.
	// One role at a time: "the directors I've seen" is the question it answers.
	let roleFilter = $derived.by(() => {
		const r = page.url.searchParams.get("role") ?? "";
		return roleCounts.some((x) => x.role === r) ? r : "";
	});

	let shownPeople = $derived(
		roleFilter
			? (people ?? []).filter((p) => p.roles.includes(roleFilter))
			: (people ?? []),
	);

	function selectRole(r: string) {
		const next = r === roleFilter ? "" : r;
		goto(
			next
				? resolve(`/live?tab=artists&role=${encodeURIComponent(next)}`)
				: resolve("/live?tab=artists"),
			{ replaceState: true, keepFocus: true, noScroll: true },
		);
	}
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
				{#if roleCounts.length > 1}
					<div class="role-filter" role="group" aria-label="Filter by role">
						<button
							type="button"
							class="chip"
							aria-pressed={!roleFilter}
							onclick={() => selectRole("")}
						>
							All <span class="n">{people.length}</span>
						</button>
						{#each roleCounts as { role, n } (role)}
							<button
								type="button"
								class="chip"
								aria-pressed={roleFilter === role}
								onclick={() => selectRole(role)}
							>
								{role} <span class="n">{n}</span>
							</button>
						{/each}
					</div>
				{/if}
				<ul class="people">
					{#each shownPeople as p (p.id)}
						<li>
							<a href={resolve(`/live/people/${p.id}`)}>
								<span class="who">
									<span class="name">{p.name}</span>
									<span class="roles">
										{#each orderRoles(p.roles) as r (r)}
											<span class="role" data-active={r === roleFilter}
												>{r}</span
											>
										{/each}
									</span>
								</span>
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

	.role-filter {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin-bottom: 10px;
	}

	.chip {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		width: auto;
		padding: 4px 12px;
		border: 1px solid $text-color-accent;
		border-radius: 14px;
		background-color: transparent;
		font-size: 13px;
		color: $text-color;

		.n {
			font-size: 11px;
			opacity: 0.7;
		}

		// Hover only tints, so on touch a stuck hover never looks pressed.
		@media (hover: hover) {
			&:hover {
				background-color: $bg-color-accent;
			}
		}

		&[aria-pressed="true"] {
			border-color: $text-color;
			background-color: $text-color;
			color: $bg-color;
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

			.who {
				display: flex;
				flex-flow: column;
				gap: 4px;
				flex: 1 1 auto;
				min-width: 0;
			}

			.name {
				font-weight: bold;
				overflow-wrap: anywhere;
			}

			.roles {
				display: flex;
				flex-wrap: wrap;
				gap: 4px;
			}

			// Read-only labels: lighter than the filter chips above, which are
			// the ones to press. The role being filtered on stands out.
			.role {
				padding: 1px 8px;
				border: 1px solid $bg-color-accent;
				border-radius: 10px;
				font-size: 12px;
				color: $text-color-accent;

				&[data-active="true"] {
					border-color: $text-color;
					color: $text-color;
				}
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
