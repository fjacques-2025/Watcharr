<script lang="ts">
	// "On stage" for the TMDB person page: the notebook's shows this person was
	// credited on, when they're linked to an artist. Renders nothing otherwise,
	// failures included — it's a bonus on someone else's page, not its subject.
	import { resolve } from "$app/paths";
	import type { LivePersonDetail, LiveShow } from "@/types";
	import LiveShowCard from "./LiveShowCard.svelte";
	import { getPersonByTmdb } from "./api";

	interface Props {
		/** Undefined (e.g. a route param still unknown) renders nothing. */
		tmdbPersonId: number | undefined;
	}

	let { tmdbPersonId }: Props = $props();

	let person: LivePersonDetail | null = $state(null);

	// Each show once (a person can hold several roles on one), latest first.
	let shows: LiveShow[] = $derived.by(() => {
		const all = (person?.roles ?? []).flatMap((r) => r.shows);
		return all
			.filter((s, i) => all.findIndex((t) => t.id === s.id) === i)
			.sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime());
	});

	$effect(() => {
		const forId = tmdbPersonId;
		person = null;
		if (!forId || !Number.isInteger(forId) || forId <= 0) return;
		getPersonByTmdb(forId)
			.then((r) => {
				if (forId === tmdbPersonId) person = r?.person ?? null;
			})
			.catch((err) => {
				console.warn("LiveOnStage: lookup failed", err);
			});
	});
</script>

{#if person && shows.length}
	<section class="on-stage">
		<div class="head">
			<h3>On stage</h3>
			<a href={resolve(`/live/people/${person.id}`)}>Artist page</a>
		</div>
		<div class="list">
			{#each shows as s (s.id)}
				<LiveShowCard show={s} />
			{/each}
		</div>
	</section>
{/if}

<style lang="scss">
	.on-stage {
		width: 100%;
		// Lined up with the poster list below (same max width).
		max-width: 1200px;
		margin: 0 auto 20px auto;
		padding: 0 20px;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: 12px;
		margin-bottom: 8px;

		a {
			font-size: 14px;
			text-decoration: underline;
		}
	}

	.list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(min(280px, 100%), 1fr));
		gap: 8px;
	}
</style>
