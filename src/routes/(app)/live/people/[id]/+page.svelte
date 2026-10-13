<script lang="ts">
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import Poster from "@/lib/poster/Poster.svelte";
	import PosterList from "@/lib/poster/PosterList.svelte";
	import ArtistSuggest from "@/lib/live/ArtistSuggest.svelte";
	import LiveShowCard from "@/lib/live/LiveShowCard.svelte";
	import { errorText, getPerson, updatePerson } from "@/lib/live/api";
	import { req } from "@/lib/util/api";
	import { notify } from "@/lib/util/notify";
	import type { LivePersonDetail, Media, PersonCreditsResponse } from "@/types";

	let id = $derived(Number(page.params.id));
	let person: LivePersonDetail | undefined = $state();
	let loadError: unknown = $state();

	// Rename.
	let renaming = $state(false);
	let newName = $state("");
	let renameError: string | undefined = $state();

	// Link to TMDB.
	let linking = $state(false);
	let linkQuery = $state("");
	let linkError: string | undefined = $state();
	let busy = $state(false);

	// "On screen": the linked TMDB person's films and series.
	let screen: Media[] | undefined = $state();
	let screenFailed = $state(false);

	async function load(forId: number) {
		loadError = undefined;
		try {
			const p = await getPerson(forId);
			// Ignore a late answer for an artist we've navigated away from.
			if (forId === id) person = p;
		} catch (err) {
			console.error("live: load artist failed", err);
			if (forId === id) loadError = err;
		}
	}

	// Also covers moving from one artist to another without leaving the route.
	$effect(() => {
		const forId = id;
		person = undefined;
		renaming = false;
		linking = false;
		load(forId);
	});

	// Follows the link: loads when there is one, and again when it changes.
	$effect(() => {
		const tmdbId = person?.tmdbPersonId;
		screen = undefined;
		screenFailed = false;
		if (tmdbId) loadScreen(tmdbId);
	});

	async function loadScreen(tmdbId: number) {
		try {
			const r = await req.get<PersonCreditsResponse>(
				`/content/person/${tmdbId}/credits`,
			);
			if (person?.tmdbPersonId !== tmdbId) return;
			// One entry per title (several roles in one film would repeat it),
			// most voted first, like the person page's default.
			screen = (r.credits ?? [])
				.filter(
					(v, i, a) => a.findIndex((t) => t.ids.tmdb === v.ids.tmdb) === i,
				)
				.sort((a, b) => (b.ratingCount ?? 0) - (a.ratingCount ?? 0));
		} catch (err) {
			console.error("live: load on-screen credits failed", err);
			if (person?.tmdbPersonId === tmdbId) screenFailed = true;
		}
	}

	function startRename() {
		if (!person) return;
		newName = person.name;
		renameError = undefined;
		renaming = true;
	}

	async function saveName(ev: SubmitEvent) {
		ev.preventDefault();
		if (!person || busy) return;
		const name = newName.trim();
		if (!name) {
			renameError = "A name is required.";
			return;
		}
		if (name === person.name) {
			renaming = false;
			return;
		}
		busy = true;
		renameError = undefined;
		try {
			const r = await updatePerson(person.id, {
				name,
				tmdbPersonId: person.tmdbPersonId,
			});
			person.name = r.name;
			person.tmdbPersonId = r.tmdbPersonId;
			renaming = false;
		} catch (err) {
			console.error("live: rename artist failed", err);
			renameError = errorText(err);
		} finally {
			busy = false;
		}
	}

	function startLink() {
		if (!person) return;
		linkQuery = person.name;
		linkError = undefined;
		linking = true;
	}

	async function setLink(tmdbPersonId: number | null) {
		if (!person || busy) return;
		busy = true;
		linkError = undefined;
		try {
			const r = await updatePerson(person.id, {
				name: person.name,
				tmdbPersonId,
			});
			person.name = r.name;
			person.tmdbPersonId = r.tmdbPersonId;
			linking = false;
		} catch (err) {
			console.error("live: link artist failed", err);
			if (tmdbPersonId === null)
				notify({ type: "error", text: `Failed to unlink: ${errorText(err)}` });
			else linkError = errorText(err);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{person?.name ?? "Artist"}</title>
</svelte:head>

<div class="content">
	<div class="inner">
		<BackButton />

		{#if loadError}
			<Error
				pretty="Failed to load this artist!"
				error={loadError}
				onRetry={() => {
					load(id);
				}}
			/>
		{:else if !person}
			<Spinner />
		{:else}
			{#if renaming}
				<form class="rename" onsubmit={saveName}>
					<input
						type="text"
						bind:value={newName}
						aria-label="Name"
						maxlength={200}
						autocomplete="off"
					/>
					<button type="submit" disabled={busy}>Save</button>
					<button
						type="button"
						class="secondary"
						onclick={() => (renaming = false)}
					>
						Cancel
					</button>
				</form>
				{#if renameError}
					<p class="error" role="alert">{renameError}</p>
				{/if}
			{:else}
				<div class="name-row">
					<h2>{person.name}</h2>
					<button class="small" onclick={startRename} disabled={busy}>
						<Icon i="pencil" wh={14} /> Edit
					</button>
				</div>
			{/if}

			<div class="tmdb">
				{#if person.tmdbPersonId}
					<a href={resolve(`/person/${person.tmdbPersonId}`)}>
						<Icon i="person" wh={16} /> Open in Watcharr
					</a>
					<button
						class="small"
						onclick={() => setLink(null)}
						disabled={busy}
						title="Remove the link to this TMDB person"
					>
						Unlink
					</button>
				{:else if linking}
					<div class="link-search">
						<p class="help">
							Pick the right person: namesakes are common, check the photo.
						</p>
						<ArtistSuggest
							bind:value={linkQuery}
							tmdbOnly
							autofocus
							placeholder="Search TMDB people"
							ariaLabel="Search TMDB people"
							onPick={(p) => setLink(p.tmdbPersonId)}
						/>
						<button
							class="small secondary"
							onclick={() => (linking = false)}
							disabled={busy}
						>
							Cancel
						</button>
					</div>
				{:else}
					<span class="muted">Not linked to TMDB.</span>
					<button class="small" onclick={startLink} disabled={busy}>
						Link to a TMDB person
					</button>
				{/if}
			</div>
			{#if linkError}
				<p class="error" role="alert">{linkError}</p>
			{/if}

			{#each person.roles as r (r.role)}
				<section>
					<h3>
						{r.role} — {r.shows.length} show{r.shows.length === 1 ? "" : "s"}
					</h3>
					<div class="list">
						{#each r.shows as s (s.id)}
							<LiveShowCard show={s} />
						{/each}
					</div>
				</section>
			{:else}
				<p class="muted">Not credited on any show.</p>
			{/each}

			{#if person.tmdbPersonId}
				<section class="screen">
					<h3>On screen</h3>
					{#if screenFailed}
						<p class="muted">Couldn't load their films and series.</p>
					{:else if !screen}
						<Spinner />
					{:else if screen.length}
						<PosterList>
							{#each screen as c, i (`${i}-${c.ids.tmdb}`)}
								<Poster media={c} bind:watched={screen[i].watched} fluidSize />
							{/each}
						</PosterList>
					{:else}
						<p class="muted">No films or series on TMDB.</p>
					{/if}
				</section>
			{/if}
		{/if}
	</div>
</div>

<style lang="scss">
	.content {
		display: flex;
		width: 100%;
		justify-content: center;
		margin-top: -18px;

		.inner {
			width: 100%;
			max-width: 800px;
			padding: 0 15px 60px 15px;
		}
	}

	button.small {
		gap: 6px;
		width: auto;
		padding: 4px 10px;
		font-size: 13px;
	}

	.name-row {
		display: flex;
		flex-flow: row wrap;
		align-items: center;
		gap: 8px 12px;

		h2 {
			min-width: 0;
			overflow-wrap: anywhere;
		}
	}

	.rename {
		display: flex;
		flex-flow: row wrap;
		gap: 8px;

		input {
			flex: 1 1 200px;
			min-width: 0;
			color: $text-color;
			background-color: transparent;
			border-color: $text-color;
		}

		button {
			width: auto;
		}
	}

	.tmdb {
		display: flex;
		flex-flow: row wrap;
		align-items: center;
		gap: 8px 12px;
		margin-top: 10px;

		a {
			display: inline-flex;
			align-items: center;
			gap: 6px;
			fill: currentColor;
			text-decoration: underline;
		}
	}

	.link-search {
		display: flex;
		flex-flow: column;
		align-items: flex-start;
		gap: 6px;
		width: 100%;
		max-width: 420px;
	}

	.help,
	.muted {
		font-size: 14px;
		color: $text-color-accent;
	}

	.error {
		margin-top: 8px;
		padding: 8px 10px;
		border-radius: 8px;
		color: white;
		background-color: $error;
	}

	section {
		margin-top: 22px;

		h3 {
			margin-bottom: 8px;
		}
	}

	.list {
		display: flex;
		flex-flow: column;
		gap: 8px;
	}

	.screen :global(ul) {
		margin-left: 0;
		margin-right: 0;
	}
</style>
